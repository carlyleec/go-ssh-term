package connections

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/auth"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

func testTerminalOwner() terminalOwner {
	return terminalOwner{accountID: uuid.NewString(), session: auth.LoginSession{ID: uuid.NewString(), ExpiresAt: time.Now().Add(time.Hour)}}
}

func TestTerminalRegistryAdmissionAndRemoval(t *testing.T) {
	r := newTerminalRegistry()
	owner := testTerminalOwner()
	for _, invalid := range []terminalOwner{{}, {accountID: owner.accountID}, {accountID: owner.accountID, session: auth.LoginSession{ID: owner.session.ID, ExpiresAt: time.Now().Add(-time.Second)}}} {
		if _, err := r.start(t.Context(), invalid, nil); !errors.Is(err, errLiveTerminal) {
			t.Fatal("invalid owner admitted")
		}
	}
	parent, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := r.start(parent, owner, nil); !errors.Is(err, errLiveTerminal) {
		t.Fatal("canceled attempt admitted")
	}
	a, err := r.start(t.Context(), owner, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.complete(a)
	b, err := r.start(t.Context(), owner, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.complete(b)
	if a.id == b.id {
		t.Fatal("terminal IDs reused")
	}
	wrongLogin := owner
	wrongLogin.session.ID = uuid.NewString()
	wrongAccount := owner
	wrongAccount.accountID = uuid.NewString()
	extended := owner
	extended.session.ExpiresAt = owner.session.ExpiresAt.Add(time.Hour)
	for _, other := range []terminalOwner{wrongLogin, wrongAccount, extended} {
		if !errors.Is(r.input(other, a.id, []byte("x")), errLiveTerminal) || !errors.Is(r.resize(other, a.id, 30, 90), errLiveTerminal) || !errors.Is(r.close(other, a.id), errLiveTerminal) || !errors.Is(r.publish(other, a.id, nil, nil, nil), errLiveTerminal) {
			t.Fatal("foreign operation accepted")
		}
	}
	if a.ctx.Err() != nil {
		t.Fatal("foreign close canceled terminal")
	}
	if err := r.close(owner, a.id); err != nil {
		t.Fatal(err)
	}
	if a.ctx.Err() == nil || b.ctx.Err() != nil {
		t.Fatal("close affected wrong terminal")
	}
	if !errors.Is(r.close(owner, a.id), errLiveTerminal) || !errors.Is(r.input(owner, "missing", nil), errLiveTerminal) {
		t.Fatal("stale or missing ID accepted")
	}
	r.release(a)
	if b.ctx.Err() != nil {
		t.Fatal("repeated cleanup affected another terminal")
	}
}

func TestTerminalRegistryExpiryAndLatePublication(t *testing.T) {
	r := newTerminalRegistry()
	owner := testTerminalOwner()
	owner.session.ExpiresAt = time.Now().Add(50 * time.Millisecond)
	entry, err := r.start(t.Context(), owner, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.complete(entry)
	select {
	case <-entry.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("deadline not enforced")
	}
	if !errors.Is(r.input(owner, entry.id, nil), errLiveTerminal) || !errors.Is(r.resize(owner, entry.id, 24, 80), errLiveTerminal) {
		t.Fatal("expired operation accepted")
	}
	if !errors.Is(r.publish(owner, entry.id, &ssh.Client{}, &ssh.Session{}, discardInput{}), errLiveTerminal) {
		t.Fatal("late client published")
	}
}

type discardInput struct{}

func (discardInput) Write(p []byte) (int, error) { return len(p), nil }
func (discardInput) Close() error                { return nil }

// The peer echoes input and reports resizes from separate goroutines. SSH
// channels require serialization of concurrent writes to the same stream.
type serializedTestWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *serializedTestWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(p)
}

func startRegistryPeer(t *testing.T, host, user ssh.Signer) *peer {
	t.Helper()
	return startPeerChannels(t, host, user, func(newChannel ssh.NewChannel) {
		channel, requests, err := newChannel.Accept()
		if err != nil {
			return
		}
		copied := make(chan struct{})
		output := &serializedTestWriter{writer: channel}
		go func() { defer close(copied); _, _ = io.Copy(output, channel) }()
		defer func() { _ = channel.Close(); <-copied }()
		for request := range requests {
			_ = request.Reply(true, nil)
			if request.Type == "window-change" {
				var size struct{ Cols, Rows, Width, Height uint32 }
				if ssh.Unmarshal(request.Payload, &size) == nil {
					_, _ = fmt.Fprintf(output, "resize %d %d", size.Cols, size.Rows)
				}
			}
		}
	})
}

func registeredTerminal(t *testing.T, d *Dialer, seen map[string]bool) *liveTerminal {
	t.Helper()
	d.terminals.mu.Lock()
	defer d.terminals.mu.Unlock()
	for id, entry := range d.terminals.entries {
		if !seen[id] {
			if entry.client == nil || entry.shell == nil || entry.stdin == nil {
				t.Fatal("connected before handle publication")
			}
			seen[id] = true
			return entry
		}
	}
	t.Fatal("live terminal not registered")
	return nil
}

func expectTerminalBytes(t *testing.T, conn *websocket.Conn, want string) {
	t.Helper()
	kind, data, err := conn.ReadMessage()
	if err != nil || kind != websocket.BinaryMessage || string(data) != want {
		t.Fatalf("want %q got %q: %v", want, data, err)
	}
}

func TestTerminalRegistryLoginIsolation(t *testing.T) {
	f, d, user := dialFixture(t)
	host, _ := newSigner(t)
	p := startRegistryPeer(t, host, user)
	saved := approvedTerminal(t, f, d, p)
	seen := map[string]bool{}
	var conns []*websocket.Conn
	var entries []*liveTerminal
	for i := 0; i < 3; i++ {
		if i == 2 {
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
		}
		conn := openTerminal(t, f, saved)
		terminalState(t, conn, "connecting")
		terminalState(t, conn, "connected")
		entry := registeredTerminal(t, d, seen)
		sum := sha256.Sum256([]byte(f.cookies[0].Value))
		if entry.id == saved.ID || entry.owner.accountID != f.owners[0] || entry.owner.session.ID != hex.EncodeToString(sum[:]) {
			t.Fatal("incorrect registry ownership")
		}
		conns = append(conns, conn)
		entries = append(entries, entry)
	}
	if entries[0].owner != entries[1].owner || entries[0].owner == entries[2].owner {
		t.Fatal("login ownership not separated")
	}
	for _, wrong := range []terminalOwner{entries[2].owner, {accountID: f.owners[1], session: entries[0].owner.session}} {
		if !errors.Is(d.terminals.input(wrong, entries[0].id, []byte("foreign")), errLiveTerminal) || !errors.Is(d.terminals.resize(wrong, entries[0].id, 30, 90), errLiveTerminal) || !errors.Is(d.terminals.close(wrong, entries[0].id), errLiveTerminal) {
			t.Fatal("foreign login controlled live shell")
		}
	}
	if err := d.terminals.publish(entries[0].owner, entries[0].id, entries[0].client, entries[0].shell, entries[0].stdin); !errors.Is(err, errLiveTerminal) {
		t.Fatal("published twice")
	}
	for i, conn := range conns {
		message := fmt.Sprintf("shell %d", i)
		if err := conn.WriteMessage(websocket.BinaryMessage, []byte(message)); err != nil {
			t.Fatal(err)
		}
		expectTerminalBytes(t, conn, message)
		if err := d.terminals.resize(entries[i].owner, entries[i].id, 30+i, 90+i); err != nil {
			t.Fatal(err)
		}
		expectTerminalBytes(t, conn, fmt.Sprintf("resize %d %d", 90+i, 30+i))
	}
	expect(t, f.request("DELETE", "/api/connections/"+saved.ID, nil, 0), 204)
	if err := d.terminals.close(entries[0].owner, entries[0].id); err != nil {
		t.Fatal(err)
	}
	if _, _, err := conns[0].ReadMessage(); err == nil {
		t.Fatal("closed terminal still open")
	}
	for _, i := range []int{1, 2} {
		if err := d.terminals.input(entries[i].owner, entries[i].id, []byte("still live")); err != nil {
			t.Fatal(err)
		}
		expectTerminalBytes(t, conns[i], "still live")
	}
	// Removal racing with input/resize must be safe and leave no stale authority.
	var work sync.WaitGroup
	for range 8 {
		work.Go(func() { _ = d.terminals.input(entries[1].owner, entries[1].id, []byte("x")) })
		work.Go(func() { _ = d.terminals.resize(entries[1].owner, entries[1].id, 24, 80) })
	}
	work.Go(func() { _ = d.terminals.close(entries[1].owner, entries[1].id) })
	work.Wait()
	if !errors.Is(d.terminals.input(entries[1].owner, entries[1].id, nil), errLiveTerminal) {
		t.Fatal("removed handle still usable")
	}
}

func TestTerminalLiveExpiry(t *testing.T) {
	f, d, user := dialFixture(t)
	host, _ := newSigner(t)
	p := startRegistryPeer(t, host, user)
	saved := approvedTerminal(t, f, d, p)
	ctx, err := f.sessions.Load(t.Context(), f.cookies[0].Value)
	if err != nil {
		t.Fatal(err)
	}
	f.sessions.SetDeadline(ctx, time.Now().Add(500*time.Millisecond))
	if _, _, err := f.sessions.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	conn := openTerminal(t, f, saved)
	terminalState(t, conn, "connecting")
	terminalState(t, conn, "connected")
	entry := registeredTerminal(t, d, map[string]bool{})
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("expiry left socket open")
	}
	if !errors.Is(d.terminals.input(entry.owner, entry.id, []byte("after expiry")), errLiveTerminal) {
		t.Fatal("expired shell still accepts input")
	}
}

func TestTerminalExpiryDuringSetup(t *testing.T) {
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
	ctx, err := f.sessions.Load(t.Context(), f.cookies[0].Value)
	if err != nil {
		t.Fatal(err)
	}
	f.sessions.SetDeadline(ctx, time.Now().Add(500*time.Millisecond))
	if _, _, err := f.sessions.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	conn := openTerminal(t, f, saved)
	terminalState(t, conn, "connecting")
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("setup not started")
	}
	d.terminals.mu.Lock()
	count := len(d.terminals.entries)
	d.terminals.mu.Unlock()
	if count != 1 {
		t.Fatal("pending setup not registered")
	}
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("expired setup still open")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expiry did not interrupt SSH setup")
	}
}
