package connections

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

func secondLogin(t *testing.T, f fixture) fixture {
	t.Helper()
	ctx, err := f.sessions.Load(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	f.sessions.Put(ctx, "account_id", f.owners[0])
	token, _, err := f.sessions.Commit(ctx)
	if err != nil {
		t.Fatal(err)
	}
	f.cookies[0] = &http.Cookie{Name: f.sessions.Cookie.Name, Value: token}
	return f
}

func TestTerminalLogoutClosesOnlyOwningLogin(t *testing.T) {
	f, d, user := dialFixture(t)
	host, _ := newSigner(t)
	p := startRegistryPeer(t, host, user)
	saved := approvedTerminal(t, f, d, p)
	seen := map[string]bool{}
	first := openTerminal(t, f, saved)
	terminalState(t, first, "connecting")
	terminalState(t, first, "connected")
	entry := registeredTerminal(t, d, seen)
	other := openTerminal(t, secondLogin(t, f), saved)
	terminalState(t, other, "connecting")
	terminalState(t, other, "connected")
	// Failed session destruction must not invalidate an otherwise live shell.
	if _, err := f.db.Exec(`CREATE TRIGGER fail_logout BEFORE DELETE ON sessions BEGIN SELECT RAISE(FAIL, 'test'); END`); err != nil {
		t.Fatal(err)
	}
	expect(t, f.request("POST", "/api/auth/logout", nil, 0), 503)
	if err := d.terminals.input(entry.owner, entry.id, []byte("still authorized")); err != nil {
		t.Fatal(err)
	}
	expectTerminalBytes(t, first, "still authorized")
	if _, err := f.db.Exec("DROP TRIGGER fail_logout"); err != nil {
		t.Fatal(err)
	}
	expect(t, f.request("POST", "/api/auth/logout", nil, 0), 204)
	select {
	case <-entry.done:
	default:
		t.Fatal("logout returned before cleanup")
	}
	if _, _, err := first.ReadMessage(); err == nil {
		t.Fatal("logout left socket open")
	}
	if _, err := d.terminals.start(t.Context(), entry.owner, nil); !errors.Is(err, errLiveTerminal) {
		t.Fatal("admitted stale login after logout")
	}
	if err := other.WriteMessage(websocket.BinaryMessage, []byte("other login")); err != nil {
		t.Fatal(err)
	}
	expectTerminalBytes(t, other, "other login")
	expect(t, f.request("POST", "/api/auth/logout", nil, 0), 204)
}

func TestTerminalLogoutInterruptsSetup(t *testing.T) {
	f, d, user := dialFixture(t)
	host, _ := newSigner(t)
	started, done := make(chan struct{}), make(chan struct{})
	p := startPeerChannels(t, host, user, func(newChannel ssh.NewChannel) {
		defer close(done)
		channel, requests, err := newChannel.Accept()
		if err != nil {
			return
		}
		defer channel.Close()
		for range requests {
			close(started)
		}
	})
	saved := approvedTerminal(t, f, d, p)
	conn := openTerminal(t, f, saved)
	terminalState(t, conn, "connecting")
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("setup not reached")
	}
	expect(t, f.request("POST", "/api/auth/logout", nil, 0), 204)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("logout left setup alive")
	}
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("late setup succeeded")
	}
}

func TestTerminalShutdownWaitsAndRejectsNewWork(t *testing.T) {
	f, d, user := dialFixture(t)
	host, _ := newSigner(t)
	p := startRegistryPeer(t, host, user)
	saved := approvedTerminal(t, f, d, p)
	conn := openTerminal(t, f, saved)
	terminalState(t, conn, "connecting")
	terminalState(t, conn, "connected")
	entry := registeredTerminal(t, d, map[string]bool{})
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := d.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entry.done:
	default:
		t.Fatal("shutdown did not join handler")
	}
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("shutdown left socket open")
	}
	if _, err := d.terminals.start(t.Context(), testTerminalOwner(), nil); !errors.Is(err, errLiveTerminal) {
		t.Fatal("admission after shutdown")
	}
	if err := d.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestTerminalRevocationRacesAdmission(t *testing.T) {
	for range 30 {
		d := NewDialer(nil, nil)
		owner := testTerminalOwner()
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Go(func() {
			<-start
			entry, err := d.terminals.start(t.Context(), owner, nil)
			if err != nil {
				return
			}
			<-entry.ctx.Done()
			if err := d.terminals.publish(owner, entry.id, &ssh.Client{}, &ssh.Session{}, discardInput{}); !errors.Is(err, errLiveTerminal) {
				t.Error("late publication after revoke")
			}
			d.terminals.complete(entry)
		})
		wg.Go(func() { <-start; d.InvalidateSession(owner.session) })
		close(start)
		wg.Wait()
		if _, err := d.terminals.start(t.Context(), owner, nil); !errors.Is(err, errLiveTerminal) {
			t.Fatal("revocation lost")
		}
	}
}

func TestTerminalSilentBrowserAndPartialMessage(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run(map[bool]string{false: "silent", true: "unfinished message"}[partial], func(t *testing.T) {
			f, d, user := dialFixture(t)
			d.pingInterval = 20 * time.Millisecond
			d.pongWait = 150 * time.Millisecond
			d.terminals.ioTimeout = 75 * time.Millisecond
			host, _ := newSigner(t)
			p := startRegistryPeer(t, host, user)
			saved := approvedTerminal(t, f, d, p)
			conn := openTerminal(t, f, saved)
			terminalState(t, conn, "connecting")
			terminalState(t, conn, "connected")
			entry := registeredTerminal(t, d, map[string]bool{})
			if partial {
				writer, err := conn.NextWriter(websocket.BinaryMessage)
				if err != nil {
					t.Fatal(err)
				}
				// Flush a continuation frame, leaving the message incomplete.
				_, _ = writer.Write(bytes.Repeat([]byte("x"), 8192))
				defer writer.Close()
			}
			// No reads means no pong processing; an abandoned browser must expire.
			select {
			case <-entry.done:
			case <-time.After(time.Second):
				t.Fatal("silent browser leaked terminal")
			}
		})
	}
}

func TestTerminalStalledSSHInput(t *testing.T) {
	f, d, user := dialFixture(t)
	d.terminals.ioTimeout = 100 * time.Millisecond
	host, _ := newSigner(t)
	p := startPeerChannels(t, host, user, func(newChannel ssh.NewChannel) {
		channel, requests, err := newChannel.Accept()
		if err != nil {
			return
		}
		defer channel.Close()
		for request := range requests {
			_ = request.Reply(true, nil)
		}
	})
	saved := approvedTerminal(t, f, d, p)
	conn := openTerminal(t, f, saved)
	terminalState(t, conn, "connecting")
	terminalState(t, conn, "connected")
	entry := registeredTerminal(t, d, map[string]bool{})
	sent := make(chan struct{})
	go func() {
		defer close(sent)
		for range 128 {
			if conn.WriteMessage(websocket.BinaryMessage, make([]byte, terminalDataLimit)) != nil {
				return
			}
		}
	}()
	select {
	case <-entry.done:
	case <-time.After(3 * time.Second):
		t.Fatal("blocked SSH input leaked terminal")
	}
	_ = conn.Close()
	<-sent
}

func TestTerminalSlowBrowserOutput(t *testing.T) {
	f, d, user := dialFixture(t)
	d.terminals.ioTimeout = 100 * time.Millisecond
	host, _ := newSigner(t)
	p := startPeerChannels(t, host, user, func(newChannel ssh.NewChannel) {
		channel, requests, err := newChannel.Accept()
		if err != nil {
			return
		}
		defer channel.Close()
		for request := range requests {
			_ = request.Reply(true, nil)
			if request.Type == "shell" {
				_, _ = io.CopyN(channel, zeroReader{}, 32*1024*1024)
				return
			}
		}
	})
	saved := approvedTerminal(t, f, d, p)
	conn := openTerminal(t, f, saved)
	terminalState(t, conn, "connecting")
	terminalState(t, conn, "connected")
	entry := registeredTerminal(t, d, map[string]bool{})
	select {
	case <-entry.done:
	case <-time.After(3 * time.Second):
		t.Fatal("slow browser leaked terminal")
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func TestTerminalSilentSSHPeer(t *testing.T) {
	f, d, user := dialFixture(t)
	d.pingInterval = 20 * time.Millisecond
	d.pongWait = time.Second
	d.terminals.ioTimeout = 100 * time.Millisecond
	host, _ := newSigner(t)
	p := startRegistryPeer(t, host, user)
	saved := approvedTerminal(t, f, d, p)
	p.ignoreRequests.Store(true)
	conn := openTerminal(t, f, saved)
	terminalState(t, conn, "connecting")
	terminalState(t, conn, "connected")
	entry := registeredTerminal(t, d, map[string]bool{})
	readDone := make(chan struct{})
	go func() { defer close(readDone); _, _, _ = conn.ReadMessage() }()
	select {
	case <-entry.done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("silent SSH peer not detected")
	}
	<-readDone
}

func TestTerminalHeartbeatsKeepIdleShellAlive(t *testing.T) {
	f, d, user := dialFixture(t)
	d.pingInterval = 25 * time.Millisecond
	d.pongWait = 150 * time.Millisecond
	d.terminals.ioTimeout = 100 * time.Millisecond
	host, _ := newSigner(t)
	p := startRegistryPeer(t, host, user)
	saved := approvedTerminal(t, f, d, p)
	conn := openTerminal(t, f, saved)
	terminalState(t, conn, "connecting")
	terminalState(t, conn, "connected")
	entry := registeredTerminal(t, d, map[string]bool{})
	pings := make(chan struct{}, 8)
	reply := conn.PingHandler()
	conn.SetPingHandler(func(payload string) error {
		select {
		case pings <- struct{}{}:
		default:
		}
		return reply(payload)
	})
	readDone := make(chan struct{})
	go func() { defer close(readDone); _, _, _ = conn.ReadMessage() }()
	for range 8 {
		select {
		case <-pings:
		case <-time.After(time.Second):
			t.Fatal("idle heartbeat failed")
		}
	}
	if entry.ctx.Err() != nil {
		t.Fatal("healthy idle shell canceled")
	}
	_ = conn.Close()
	<-readDone
}
