package connections

import (
	"bytes"
	"context"
	"encoding/pem"
	"io"
	"mime/multipart"
	"net"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func uploadJumpKey(t *testing.T, f fixture) (ssh.Signer, string) {
	t.Helper()
	signer, key := newSigner(t)
	block, err := ssh.MarshalPrivateKey(key, "")
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("name", "Target key")
	part, err := writer.CreateFormFile("private_key", "key")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(pem.EncodeToMemory(block))
	_ = writer.Close()
	request := httptest.NewRequest("POST", "/api/keys", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Origin", origin)
	request.AddCookie(f.cookies[0])
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, request)
	if response.Code != 201 {
		t.Fatalf("target key upload: %d", response.Code)
	}
	var id string
	if err := f.db.QueryRow("SELECT id FROM ssh_keys WHERE name='Target key'").Scan(&id); err != nil {
		t.Fatal(err)
	}
	return signer, id
}

type jumpLab struct {
	f                           fixture
	d                           *Dialer
	target, bastion             *peer
	targetConfig, bastionConfig Connection
	active                      atomic.Int64
	requested                   atomic.Int64
	reject                      atomic.Bool
	hold                        atomic.Bool
	silent                      atomic.Bool
	opened                      chan struct{}
	release                     chan struct{}
}

func newJumpLab(t *testing.T) *jumpLab {
	t.Helper()
	f, d, user := dialFixture(t)
	targetUser, keyID := uploadJumpKey(t, f)
	targetHost, _ := newSigner(t)
	jumpHost, _ := newSigner(t)
	lab := &jumpLab{f: f, d: d, opened: make(chan struct{}, 20), release: make(chan struct{})}
	t.Cleanup(func() { close(lab.release) })
	lab.target = startRegistryPeer(t, targetHost, targetUser)
	lab.bastion = startPeerChannels(t, jumpHost, user, func(incoming ssh.NewChannel) {
		if incoming.ChannelType() != "direct-tcpip" {
			_ = incoming.Reject(ssh.Prohibited, "unexpected channel")
			return
		}
		var payload struct {
			Host       string
			Port       uint32
			Origin     string
			OriginPort uint32
		}
		if ssh.Unmarshal(incoming.ExtraData(), &payload) != nil || payload.Host != "private-target.invalid" || payload.Port != 2222 {
			t.Error("wrong forwarding destination")
			_ = incoming.Reject(ssh.Prohibited, "wrong destination")
			return
		}
		lab.requested.Add(1)
		lab.opened <- struct{}{}
		if lab.hold.Load() {
			select {
			case <-lab.release:
			case <-t.Context().Done():
			}
		}
		if lab.reject.Load() {
			_ = incoming.Reject(ssh.ConnectionFailed, "private server details")
			return
		}
		if lab.silent.Load() {
			channel, requests, err := incoming.Accept()
			if err != nil {
				return
			}
			defer channel.Close()
			lab.active.Add(1)
			defer lab.active.Add(-1)
			done := make(chan struct{})
			go func() { ssh.DiscardRequests(requests); close(done) }()
			_, _ = io.Copy(io.Discard, channel)
			<-done
			return
		}
		upstream, err := net.Dial("tcp", lab.target.listener.Addr().String())
		if err != nil {
			_ = incoming.Reject(ssh.ConnectionFailed, "unreachable")
			return
		}
		defer upstream.Close()
		channel, requests, err := incoming.Accept()
		if err != nil {
			return
		}
		defer channel.Close()
		lab.active.Add(1)
		defer lab.active.Add(-1)
		var workers sync.WaitGroup
		workers.Go(func() { ssh.DiscardRequests(requests) })
		copied := make(chan struct{}, 2)
		workers.Go(func() { _, _ = io.Copy(upstream, channel); copied <- struct{}{} })
		workers.Go(func() { _, _ = io.Copy(channel, upstream); copied <- struct{}{} })
		<-copied
		_ = upstream.Close()
		_ = channel.Close()
		workers.Wait()
	})
	lab.bastionConfig = approvedTerminal(t, f, d, lab.bastion)
	fields := f.fields()
	fields.Host = "private-target.invalid"
	fields.Port = 2222
	fields.SSHKeyID = keyID
	fields.JumpConnectionID = &lab.bastionConfig.ID
	lab.targetConfig = result(t, f.request("POST", "/api/connections", fields, 0), 201)
	return lab
}
func (lab *jumpLab) approveTarget(t *testing.T) {
	t.Helper()
	seen, err := lab.d.Inspect(t.Context(), lab.f.owners[0], lab.targetConfig.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lab.d.Approve(t.Context(), lab.f.owners[0], lab.targetConfig.ID, decision(seen)); err != nil {
		t.Fatal(err)
	}
	if lab.target.auths.Load() != 0 {
		t.Fatal("target authenticated during inspection/approval")
	}
	if lab.bastion.auths.Load() == 0 {
		t.Fatal("target probe bypassed bastion")
	}
}
func (lab *jumpLab) awaitActive(t *testing.T, count int64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for lab.active.Load() != count {
		if time.Now().After(deadline) {
			t.Fatalf("forwarded connections: %d want %d", lab.active.Load(), count)
		}
		time.Sleep(time.Millisecond)
	}
}
func TestJumpTerminalCredentialsAndIndependentCleanup(t *testing.T) {
	lab := newJumpLab(t)
	lab.approveTarget(t)
	lab.awaitActive(t, 0)
	seen := map[string]bool{}
	first := openTerminal(t, lab.f, lab.targetConfig)
	terminalState(t, first, "connecting")
	terminalState(t, first, "connected")
	one := registeredTerminal(t, lab.d, seen)
	second := openTerminal(t, lab.f, lab.targetConfig)
	terminalState(t, second, "connecting")
	terminalState(t, second, "connected")
	two := registeredTerminal(t, lab.d, seen)
	lab.awaitActive(t, 2)
	if err := lab.d.terminals.input(one.owner, one.id, []byte("first")); err != nil {
		t.Fatal(err)
	}
	expectTerminalBytes(t, first, "first")
	if err := lab.d.terminals.close(one.owner, one.id); err != nil {
		t.Fatal(err)
	}
	awaitTerminalCleanup(t, one)
	lab.awaitActive(t, 1)
	if err := lab.d.terminals.input(two.owner, two.id, []byte("second survives")); err != nil {
		t.Fatal(err)
	}
	expectTerminalBytes(t, second, "second survives")
	expect(t, lab.f.request("POST", "/api/auth/logout", nil, 0), 204)
	awaitTerminalCleanup(t, two)
	lab.awaitActive(t, 0)
	if lab.target.auths.Load() == 0 {
		t.Fatal("target never authenticated with its separate key")
	}
}
func TestJumpHostTrustOnBothHops(t *testing.T) {
	for _, hop := range []string{"bastion", "target"} {
		t.Run(hop, func(t *testing.T) {
			lab := newJumpLab(t)
			lab.approveTarget(t)
			lab.awaitActive(t, 0)
			beforeB, beforeT := lab.bastion.auths.Load(), lab.target.auths.Load()
			changed, _ := newSigner(t)
			peer := lab.target
			if hop == "bastion" {
				peer = lab.bastion
			}
			config := ssh.ServerConfig{PublicKeyCallback: peer.config.Load().PublicKeyCallback}
			config.AddHostKey(changed)
			peer.config.Store(&config)
			client, _, err := lab.d.Dial(t.Context(), lab.f.owners[0], lab.targetConfig.ID)
			if client != nil {
				client.Close()
				t.Fatal("changed host accepted")
			}
			requireStatus(t, err, 409)
			if lab.target.auths.Load() != beforeT {
				t.Fatal("target authenticated after changed-key rejection")
			}
			if hop == "bastion" && lab.bastion.auths.Load() != beforeB {
				t.Fatal("bastion authenticated before trust")
			}
			lab.awaitActive(t, 0)
		})
	}
}
func TestJumpForwardingFailureAndDeadline(t *testing.T) {
	for _, mode := range []string{"reject", "timeout", "cancel", "handshake"} {
		t.Run(mode, func(t *testing.T) {
			lab := newJumpLab(t)
			lab.approveTarget(t)
			lab.awaitActive(t, 0)
			for len(lab.opened) > 0 {
				<-lab.opened
			}
			if mode == "reject" {
				lab.reject.Store(true)
			} else if mode == "handshake" {
				lab.silent.Store(true)
			} else {
				lab.hold.Store(true)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				client, _, err := lab.d.Dial(ctx, lab.f.owners[0], lab.targetConfig.ID)
				if client != nil {
					client.Close()
				}
				done <- err
			}()
			select {
			case <-lab.opened:
			case <-time.After(time.Second):
				t.Fatal("forwarding not reached")
			}
			if mode == "cancel" {
				cancel()
			}
			select {
			case err := <-done:
				status := 502
				if mode != "reject" {
					status = 504
				}
				requireStatus(t, err, status)
			case <-time.After(time.Second):
				t.Fatal("forwarding did not cancel")
			}
			// Unblock the controlled peer after the client has already returned.
			lab.hold.Store(false)
			if mode == "timeout" || mode == "cancel" {
				lab.release <- struct{}{}
			}
			lab.awaitActive(t, 0)
		})
	}
}
