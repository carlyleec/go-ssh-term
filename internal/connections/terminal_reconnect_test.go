package connections

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

func awaitTerminalCleanup(t *testing.T, entry *liveTerminal) {
	t.Helper()
	select {
	case <-entry.done:
	case <-time.After(3 * time.Second):
		t.Fatal("terminal cleanup did not finish")
	}
}

func assertAttemptHistory(t *testing.T, f fixture, attempts map[string][]string) {
	t.Helper()
	events := auditEvents(t, f, 0)
	got := map[string][]string{}
	for _, event := range events {
		if event.account != f.owners[0] || event.at <= 0 {
			t.Fatalf("invalid audit identity: %+v", event)
		}
		got[event.attempt] = append(got[event.attempt], event.kind)
	}
	if !reflect.DeepEqual(got, attempts) {
		t.Fatalf("audit histories: got %v, want %v", got, attempts)
	}
}

func TestTerminalRepeatedReconnectHistory(t *testing.T) {
	f, d, user := dialFixture(t)
	host, _ := newSigner(t)
	exits := make(chan chan uint32, 1)
	p := startPeerChannels(t, host, user, func(incoming ssh.NewChannel) {
		channel, requests, err := incoming.Accept()
		if err != nil {
			return
		}
		defer channel.Close()
		for request := range requests {
			_ = request.Reply(true, nil)
			if request.Type == "shell" {
				exit := make(chan uint32, 1)
				select {
				case exits <- exit:
				case <-t.Context().Done():
					return
				}
				select {
				case status := <-exit:
					_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
				case <-t.Context().Done():
				}
				return
			}
		}
	})
	saved := approvedTerminal(t, f, d, p)
	seen := map[string]bool{}
	histories := map[string][]string{}
	for cycle := range 6 {
		conn := openTerminal(t, f, saved)
		terminalState(t, conn, "connecting")
		terminalState(t, conn, "connected")
		entry := registeredTerminal(t, d, seen)
		var exit chan uint32
		select {
		case exit = <-exits:
		case <-time.After(time.Second):
			t.Fatal("shell not started")
		}
		status := uint32(cycle % 2)
		exit <- status
		terminalState(t, conn, "disconnected")
		terminalClosed(t, conn)
		awaitTerminalCleanup(t, entry)
		histories[entry.id] = []string{"start", "end"}
		if status != 0 {
			histories[entry.id] = []string{"start", "failure", "end"}
		}
		assertAttemptHistory(t, f, histories)
		if err := d.terminals.input(entry.owner, entry.id, []byte("stale")); err == nil {
			t.Fatal("ended attempt accepted input")
		}
	}
}

func TestTerminalReplacementDialInvalidation(t *testing.T) {
	for _, reason := range []string{"logout", "expiry"} {
		t.Run(reason, func(t *testing.T) {
			f, d, user := dialFixture(t)
			host, _ := newSigner(t)
			p := startRegistryPeer(t, host, user)
			saved := approvedTerminal(t, f, d, p)
			first := openTerminal(t, f, saved)
			terminalState(t, first, "connecting")
			terminalState(t, first, "connected")
			old := registeredTerminal(t, d, map[string]bool{})
			_ = first.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
			awaitTerminalCleanup(t, old)
			// Hold replacement authentication so invalidation wins before shell publication.
			started, release := make(chan struct{}), make(chan struct{})
			var releaseOnce, startedOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(unblock)
			config := *p.config.Load()
			authenticate := config.PublicKeyCallback
			config.PublicKeyCallback = func(meta ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
				startedOnce.Do(func() { close(started) })
				select {
				case <-release:
				case <-t.Context().Done():
				}
				return authenticate(meta, key)
			}
			p.config.Store(&config)
			if reason == "expiry" {
				ctx, err := f.sessions.Load(t.Context(), f.cookies[0].Value)
				if err != nil {
					t.Fatal(err)
				}
				f.sessions.SetDeadline(ctx, time.Now().Add(750*time.Millisecond))
				if _, _, err := f.sessions.Commit(ctx); err != nil {
					t.Fatal(err)
				}
			}
			replacement := openTerminal(t, f, saved)
			terminalState(t, replacement, "connecting")
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("replacement did not reach authentication")
			}
			d.terminals.mu.Lock()
			var pending *liveTerminal
			for _, entry := range d.terminals.entries {
				pending = entry
			}
			d.terminals.mu.Unlock()
			if pending == nil || pending.id == old.id {
				t.Fatal("replacement did not get a distinct attempt")
			}
			if reason == "logout" {
				expect(t, f.request("POST", "/api/auth/logout", nil, 0), 204)
			}
			awaitTerminalCleanup(t, pending)
			unblock()
			if _, _, err := replacement.ReadMessage(); err == nil {
				t.Fatal("invalidated replacement published a status or data")
			}
			if _, err := d.terminals.start(t.Context(), pending.owner, nil); err == nil {
				t.Fatal("invalidated owner admitted another replacement")
			}
			assertAttemptHistory(t, f, map[string][]string{old.id: {"start", "end"}, pending.id: {"start", "end"}})
		})
	}
}

func TestTerminalConcurrentCancellationHasOneEnd(t *testing.T) {
	f, d, user := dialFixture(t)
	host, _ := newSigner(t)
	p := startRegistryPeer(t, host, user)
	saved := approvedTerminal(t, f, d, p)
	conn := openTerminal(t, f, saved)
	terminalState(t, conn, "connecting")
	terminalState(t, conn, "connected")
	entry := registeredTerminal(t, d, map[string]bool{})
	gate := make(chan struct{})
	var work sync.WaitGroup
	for range 4 {
		work.Go(func() { <-gate; d.InvalidateSession(entry.owner.session) })
		work.Go(func() { <-gate; _ = d.terminals.close(entry.owner, entry.id) })
		work.Go(func() {
			<-gate
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			if err := d.Shutdown(ctx); err != nil {
				t.Error(err)
			}
		})
	}
	close(gate)
	work.Wait()
	awaitTerminalCleanup(t, entry)
	assertAttemptHistory(t, f, map[string][]string{entry.id: {"start", "end"}})
	d.terminals.mu.Lock()
	remaining := len(d.terminals.entries)
	d.terminals.mu.Unlock()
	if remaining != 0 {
		t.Fatal("canceled terminal retained in registry")
	}
}
