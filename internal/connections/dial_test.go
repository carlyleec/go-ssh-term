package connections

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/api"
	"github.com/carlyleec/go-ssh-term/internal/auth"
	"github.com/carlyleec/go-ssh-term/internal/sshkeys"
	"golang.org/x/crypto/ssh"
)

type peer struct {
	listener net.Listener
	config   atomic.Pointer[ssh.ServerConfig]
	auths    atomic.Int64
}

func newSigner(t *testing.T) (ssh.Signer, ed25519.PrivateKey) {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	return s, private
}
func (p *peer) useKey(host, user ssh.Signer) {
	config := &ssh.ServerConfig{PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if bytes.Equal(key.Marshal(), user.PublicKey().Marshal()) {
			return nil, nil
		}
		return nil, errors.New("denied")
	}, AuthLogCallback: func(_ ssh.ConnMetadata, _ string, _ error) { p.auths.Add(1) }}
	config.AddHostKey(host)
	p.config.Store(config)
}
func startPeer(t *testing.T, host, user ssh.Signer) *peer {
	return startPeerChannels(t, host, user, nil)
}

func startPeerChannels(t *testing.T, host, user ssh.Signer, serve func(ssh.NewChannel)) *peer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &peer{listener: listener}
	p.useKey(host, user)
	var wg sync.WaitGroup
	var mu sync.Mutex
	sockets := map[net.Conn]bool{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			sockets[conn] = true
			mu.Unlock()
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer conn.Close()
				defer func() { mu.Lock(); delete(sockets, conn); mu.Unlock() }()
				server, chans, requests, err := ssh.NewServerConn(conn, p.config.Load())
				if err != nil {
					return
				}
				channelsDone := make(chan struct{})
				defer func() { _ = server.Close(); <-channelsDone }()
				go func() {
					defer close(channelsDone)
					for channel := range chans {
						if serve != nil {
							serve(channel)
						} else {
							_ = channel.Reject(ssh.Prohibited, "no shell in test")
						}
					}
				}()
				for request := range requests {
					_ = request.Reply(true, nil)
				}
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-done
		mu.Lock()
		for conn := range sockets {
			_ = conn.Close()
		}
		mu.Unlock()
		wg.Wait()
	})
	return p
}
func dialFixture(t *testing.T) (fixture, *Dialer, ssh.Signer) {
	t.Helper()
	f := setup(t)
	if _, err := f.db.Exec("DELETE FROM ssh_keys"); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	encryption, err := sshkeys.OpenEncryption(t.Context(), f.db, filepath.Join(dir, "application.key"))
	if err != nil {
		t.Fatal(err)
	}
	signer, private := newSigner(t)
	block, err := ssh.MarshalPrivateKey(private, "")
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err = writer.WriteField("name", "Dial key"); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("private_key", "key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write(pem.EncodeToMemory(block)); err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	mux := http.NewServeMux()
	contract := api.New(mux)
	access := auth.NewAccess(f.sessions, f.db, "localhost", origin)
	sshkeys.NewHandler(f.db, encryption).Register(contract, access)
	d := NewDialer(f.db, encryption)
	t.Cleanup(func() {
		d.terminals.mu.Lock()
		defer d.terminals.mu.Unlock()
		if len(d.terminals.entries) != 0 {
			t.Errorf("terminal registry retained %d entries", len(d.terminals.entries))
		}
	})
	d.Register(contract, access)
	d.RegisterTerminal(mux, access, origin)
	NewHandler(f.db).Register(contract, access)
	f.handler = mux
	r := httptest.NewRequest("POST", "/api/keys", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.Header.Set("Origin", origin)
	r.AddCookie(f.cookies[0])
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	if err = f.db.QueryRow("SELECT id FROM ssh_keys").Scan(&f.keys[0]); err != nil {
		t.Fatal(err)
	}
	return f, d, signer
}
func destination(t *testing.T, f fixture, p *peer) Connection {
	t.Helper()
	host, port, _ := net.SplitHostPort(p.listener.Addr().String())
	number, _ := strconv.ParseInt(port, 10, 64)
	fields := f.fields()
	fields.Host = host
	fields.Port = number
	return result(t, f.request("POST", "/api/connections", fields, 0), 201)
}
func decision(v HostInspection) HostDecision {
	return HostDecision{Host: v.Host, Port: v.Port, Fingerprint: v.Fingerprint}
}
func requireStatus(t *testing.T, err error, status int) {
	t.Helper()
	var safe *ConnectionErrorBody
	if !errors.As(err, &safe) || safe.status != status {
		t.Fatalf("want %d, got %v", status, err)
	}
}

func TestHostApprovalAndStrictDial(t *testing.T) {
	f, d, user := dialFixture(t)
	host, _ := newSigner(t)
	p := startPeer(t, host, user)
	saved := destination(t, f, p)
	observed, err := d.Inspect(t.Context(), f.owners[0], saved.ID)
	if err != nil || observed.State != "unknown" {
		t.Fatalf("inspect: %+v %v", observed, err)
	}
	if observed.Fingerprint != ssh.FingerprintSHA256(host.PublicKey()) {
		t.Fatal("wrong displayed fingerprint")
	}
	_, _, err = d.Dial(t.Context(), f.owners[0], saved.ID)
	requireStatus(t, err, 409)
	if p.auths.Load() != 0 {
		t.Fatal("unapproved host reached authentication")
	}
	_, err = d.Approve(t.Context(), f.owners[1], saved.ID, decision(observed))
	requireStatus(t, err, 404)
	bad := decision(observed)
	bad.Fingerprint = "SHA256:wrong"
	_, err = d.Approve(t.Context(), f.owners[0], saved.ID, bad)
	requireStatus(t, err, 409)
	approved, err := d.Approve(t.Context(), f.owners[0], saved.ID, decision(observed))
	if err != nil || approved.State != "trusted" {
		t.Fatalf("approve: %+v %v", approved, err)
	}
	if p.auths.Load() != 0 {
		t.Fatal("inspection or approval authenticated")
	}
	// Same-endpoint configurations share trust; another owner does not.
	other := destination(t, f, p)
	view, err := d.Inspect(t.Context(), f.owners[0], other.ID)
	if err != nil || view.State != "trusted" {
		t.Fatal("endpoint trust not shared")
	}
	client, snapshot, err := d.Dial(t.Context(), f.owners[0], saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ID != saved.ID || p.auths.Load() == 0 {
		t.Fatal("dial did not authenticate")
	}
	ok, _, err := client.SendRequest("test", true, nil)
	if err != nil || !ok {
		t.Fatal("client is unusable after setup context is released")
	}
	_ = client.Close()
	replacement, _ := newSigner(t)
	p.useKey(replacement, user)
	count := p.auths.Load()
	changed, err := d.Inspect(t.Context(), f.owners[0], saved.ID)
	if err != nil || changed.State != "changed" || changed.TrustedFingerprint != observed.Fingerprint {
		t.Fatalf("changed: %+v %v", changed, err)
	}
	_, _, err = d.Dial(t.Context(), f.owners[0], saved.ID)
	requireStatus(t, err, 409)
	_, err = d.Approve(t.Context(), f.owners[0], saved.ID, decision(changed))
	requireStatus(t, err, 409)
	if p.auths.Load() != count {
		t.Fatal("changed host reached authentication")
	}
	requireStatus(t, d.ResetTrust(t.Context(), f.owners[0], saved.ID, decision(changed)), 409)
	if err = d.ResetTrust(t.Context(), f.owners[0], saved.ID, HostDecision{Host: changed.Host, Port: changed.Port, Fingerprint: changed.TrustedFingerprint}); err != nil {
		t.Fatal(err)
	}
	view, err = d.Inspect(t.Context(), f.owners[0], saved.ID)
	if err != nil || view.State != "unknown" {
		t.Fatal("reset did not remove trust")
	}
	if _, err = d.Approve(t.Context(), f.owners[0], saved.ID, decision(view)); err != nil {
		t.Fatal(err)
	}
	client, _, err = d.Dial(t.Context(), f.owners[0], saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	_ = client.Close()
}

func TestApprovalRechecksFingerprintAndDestination(t *testing.T) {
	f, d, user := dialFixture(t)
	host, _ := newSigner(t)
	p := startPeer(t, host, user)
	saved := destination(t, f, p)
	seen, err := d.Inspect(t.Context(), f.owners[0], saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	replacement, _ := newSigner(t)
	p.useKey(replacement, user)
	_, err = d.Approve(t.Context(), f.owners[0], saved.ID, decision(seen))
	requireStatus(t, err, 409)
	if _, err = f.db.Exec("UPDATE saved_connections SET host = 'localhost' WHERE id = ?", saved.ID); err != nil {
		t.Fatal(err)
	}
	_, err = d.Approve(t.Context(), f.owners[0], saved.ID, decision(seen))
	requireStatus(t, err, 409)
	var count int
	if err = f.db.QueryRow("SELECT count(*) FROM host_trust").Scan(&count); err != nil || count != 0 {
		t.Fatal("stale approval persisted")
	}
}

func TestSSHSetupCancellationAndTimeout(t *testing.T) {
	f, d, _ := dialFixture(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	p := &peer{listener: listener}
	saved := destination(t, f, p)
	closed := make(chan struct{}, 2)
	go func() {
		for i := 0; i < 2; i++ {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				buffer := make([]byte, 256)
				for {
					if _, err := conn.Read(buffer); err != nil {
						closed <- struct{}{}
						return
					}
				}
			}()
		}
	}()
	d.timeout = 100 * time.Millisecond
	start := time.Now()
	_, err = d.Inspect(t.Context(), f.owners[0], saved.ID)
	requireStatus(t, err, 504)
	if time.Since(start) > time.Second {
		t.Fatal("deadline was not bounded")
	}
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(25*time.Millisecond, cancel)
	_, err = d.Inspect(ctx, f.owners[0], saved.ID)
	requireStatus(t, err, 504)
	for i := 0; i < 2; i++ {
		select {
		case <-closed:
		case <-time.After(time.Second):
			t.Fatal("stalled peer socket not closed")
		}
	}
}

func TestTrustHTTPGuardsAndReset(t *testing.T) {
	f, d, user := dialFixture(t)
	host, _ := newSigner(t)
	p := startPeer(t, host, user)
	saved := destination(t, f, p)
	for _, path := range []string{"host-key", "host-trust", "host-trust/reset"} {
		expect(t, f.request("POST", "/api/connections/"+saved.ID+"/"+path, HostDecision{Host: saved.Host, Port: saved.Port, Fingerprint: "x"}, -1), 401)
		expect(t, f.request("POST", "/api/connections/"+saved.ID+"/"+path, HostDecision{Host: saved.Host, Port: saved.Port, Fingerprint: "x"}, 1), 404)
		r := httptest.NewRequest("POST", "/api/connections/"+saved.ID+"/"+path, &unreadBody{t: t})
		r.AddCookie(f.cookies[0])
		r.Header.Set("Origin", "https://wrong.example")
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, r)
		expect(t, w, 403)
	}
	expect(t, f.request("POST", "/api/connections/"+saved.ID+"/host-key", nil, 0), 200)
	seen, err := d.Inspect(t.Context(), f.owners[0], saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	expect(t, f.request("POST", "/api/connections/"+saved.ID+"/host-trust", decision(seen), 0), 200)
	expect(t, f.request("POST", "/api/connections/"+saved.ID+"/host-trust/reset", decision(seen), 0), 204)
	if p.auths.Load() != 0 {
		t.Fatal("trust endpoints authenticated")
	}
}

func TestSSHAuthenticationFailureIsSafe(t *testing.T) {
	f, d, user := dialFixture(t)
	host, _ := newSigner(t)
	p := startPeer(t, host, user)
	saved := destination(t, f, p)
	seen, err := d.Inspect(t.Context(), f.owners[0], saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Approve(t.Context(), f.owners[0], saved.ID, decision(seen)); err != nil {
		t.Fatal(err)
	}
	wrong, _ := newSigner(t)
	p.useKey(host, wrong)
	client, _, err := d.Dial(t.Context(), f.owners[0], saved.ID)
	requireStatus(t, err, 502)
	if client != nil {
		client.Close()
		t.Fatal("failed authentication returned a client")
	}
	if err.Error() != "SSH handshake or authentication failed" {
		t.Fatal("raw SSH failure leaked")
	}
}

func TestHostTrustIsAccountScoped(t *testing.T) {
	f, d, user := dialFixture(t)
	host, _ := newSigner(t)
	p := startPeer(t, host, user)
	saved := destination(t, f, p)
	seen, err := d.Inspect(t.Context(), f.owners[0], saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Approve(t.Context(), f.owners[0], saved.ID, decision(seen)); err != nil {
		t.Fatal(err)
	}
	if _, err = f.db.Exec(`INSERT INTO ssh_keys VALUES (?, ?, 'Other', 'SHA256:other', X'01ff', 1)`, f.keys[1], f.owners[1]); err != nil {
		t.Fatal(err)
	}
	fields := f.fields()
	fields.Host = saved.Host
	fields.Port = saved.Port
	fields.SSHKeyID = f.keys[1]
	other := result(t, f.request("POST", "/api/connections", fields, 1), 201)
	otherView, err := d.Inspect(t.Context(), f.owners[1], other.ID)
	if err != nil || otherView.State != "unknown" {
		t.Fatal("trust crossed account boundary")
	}
	_, _, err = d.Dial(t.Context(), f.owners[1], other.ID)
	requireStatus(t, err, 409)
}
