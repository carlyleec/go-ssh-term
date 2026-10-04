package connections

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

func approvedTerminal(t *testing.T, f fixture, d *Dialer, p *peer) Connection {
	t.Helper()
	saved := destination(t, f, p)
	observed, err := d.Inspect(t.Context(), f.owners[0], saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Approve(t.Context(), f.owners[0], saved.ID, decision(observed)); err != nil {
		t.Fatal(err)
	}
	return saved
}

func openTerminal(t *testing.T, f fixture, saved Connection) *websocket.Conn {
	t.Helper()
	var handlers sync.WaitGroup
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlers.Add(1)
		defer handlers.Done()
		f.handler.ServeHTTP(w, r)
	}))
	dialer := websocket.Dialer{Subprotocols: []string{terminalProtocol}, HandshakeTimeout: time.Second}
	conn, _, err := dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/connections/"+saved.ID+"/terminal", http.Header{"Origin": {origin}, "Cookie": {f.cookies[0].String()}})
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close(); server.Close(); handlers.Wait() })
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	return conn
}

func terminalState(t *testing.T, conn *websocket.Conn, state string) terminalStatus {
	t.Helper()
	kind, data, err := conn.ReadMessage()
	var status terminalStatus
	if err != nil || kind != websocket.TextMessage || json.Unmarshal(data, &status) != nil || status.Type != "status" || status.State != state {
		t.Fatalf("want %s, got %s: %v", state, data, err)
	}
	return status
}

func terminalClosed(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	_, _, err := conn.ReadMessage()
	if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
		t.Fatalf("close: %v", err)
	}
}

func TestTerminalShellDataResizeAndExit(t *testing.T) {
	f, d, user := dialFixture(t)
	host, _ := newSigner(t)
	requestsSeen := make(chan *ssh.Request, 8)
	finish := make(chan struct{})
	peerDone := make(chan struct{})
	payload := []byte{'l', 's', 13, 3, 27, 0xff, 0, 0xe2, 0x82, 0xac}
	p := startPeerChannels(t, host, user, func(newChannel ssh.NewChannel) {
		defer close(peerDone)
		if newChannel.ChannelType() != "session" {
			t.Error("wrong channel type")
			return
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			t.Error(err)
			return
		}
		defer channel.Close()
		ioDone := make(chan struct{})
		started := false
		defer func() {
			channel.Close()
			if started {
				<-ioDone
			}
		}()
		for request := range requests {
			requestsSeen <- request
			_ = request.Reply(true, nil)
			if request.Type == "shell" {
				started = true
				go func() {
					defer close(ioDone)
					input := make([]byte, len(payload))
					if _, err := io.ReadFull(channel, input); err != nil {
						return
					}
					if !bytes.Equal(input, payload) {
						t.Error("input bytes changed")
					}
					_, _ = channel.Write(input)
					_, _ = channel.Stderr().Write([]byte("stderr"))
					select {
					case <-finish:
					case <-t.Context().Done():
						return
					}
					_, _ = channel.Write(bytes.Repeat([]byte("x"), terminalDataLimit*2+1))
					_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
					_ = channel.Close()
				}()
			}
		}
	})
	saved := approvedTerminal(t, f, d, p)
	conn := openTerminal(t, f, saved)
	terminalState(t, conn, "connecting")
	terminalState(t, conn, "connected")
	var pty struct {
		Term                      string
		Cols, Rows, Width, Height uint32
		Modes                     string
	}
	request := <-requestsSeen
	if request.Type != "pty-req" || ssh.Unmarshal(request.Payload, &pty) != nil || pty.Term != "xterm-256color" || pty.Cols != 80 || pty.Rows != 24 {
		t.Fatalf("PTY: %+v", pty)
	}
	if request := <-requestsSeen; request.Type != "shell" || len(request.Payload) != 0 {
		t.Fatal("not an interactive shell")
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, payload); err != nil {
		t.Fatal(err)
	}
	var received []byte
	for len(received) < len(payload)+len("stderr") {
		kind, data, err := conn.ReadMessage()
		if err != nil || kind != websocket.BinaryMessage {
			t.Fatalf("output: %v", err)
		}
		received = append(received, data...)
	}
	if !bytes.Contains(received, payload) || !bytes.Contains(received, []byte("stderr")) {
		t.Fatal("stdout/stderr lost")
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"resize","cols":132,"rows":43}`)); err != nil {
		t.Fatal(err)
	}
	select {
	case request := <-requestsSeen:
		var size struct{ Cols, Rows, Width, Height uint32 }
		if request.Type != "window-change" || ssh.Unmarshal(request.Payload, &size) != nil || size.Cols != 132 || size.Rows != 43 {
			t.Fatalf("resize: %+v", size)
		}
	case <-time.After(time.Second):
		t.Fatal("resize not forwarded")
	}
	// A running shell uses its original connection, independent of saved config.
	expect(t, f.request("DELETE", "/api/connections/"+saved.ID, nil, 0), 204)
	close(finish)
	var tail []byte
	for len(tail) < terminalDataLimit*2+1 {
		kind, data, err := conn.ReadMessage()
		if err != nil || kind != websocket.BinaryMessage || len(data) > terminalDataLimit {
			t.Fatalf("tail: %d %v", len(data), err)
		}
		tail = append(tail, data...)
	}
	if !bytes.Equal(tail, bytes.Repeat([]byte("x"), terminalDataLimit*2+1)) {
		t.Fatal("tail output lost")
	}
	terminalState(t, conn, "disconnected")
	terminalClosed(t, conn)
	select {
	case <-peerDone:
	case <-time.After(time.Second):
		t.Fatal("SSH channel not released")
	}
}

func TestTerminalShellSetupFailures(t *testing.T) {
	for _, stage := range []string{"channel", "pty-req", "shell", "stalled pty"} {
		t.Run(stage, func(t *testing.T) {
			f, d, user := dialFixture(t)
			host, _ := newSigner(t)
			peerDone := make(chan struct{})
			p := startPeerChannels(t, host, user, func(newChannel ssh.NewChannel) {
				defer close(peerDone)
				if stage == "channel" {
					_ = newChannel.Reject(ssh.Prohibited, "secret remote detail")
					return
				}
				channel, requests, err := newChannel.Accept()
				if err != nil {
					return
				}
				defer channel.Close()
				for request := range requests {
					if stage == "stalled pty" {
						continue
					}
					_ = request.Reply(request.Type != stage, nil)
				}
			})
			saved := approvedTerminal(t, f, d, p)
			if stage == "stalled pty" {
				d.timeout = 100 * time.Millisecond
			}
			conn := openTerminal(t, f, saved)
			terminalState(t, conn, "connecting")
			status := terminalState(t, conn, "failed")
			if strings.Contains(status.Message, "secret") {
				t.Fatal("remote error leaked")
			}
			terminalClosed(t, conn)
			select {
			case <-peerDone:
			case <-time.After(time.Second):
				t.Fatal("failed setup leaked SSH channel")
			}
		})
	}
}

func TestTerminalBrowserCloseReleasesShell(t *testing.T) {
	f, d, user := dialFixture(t)
	host, _ := newSigner(t)
	done := make(chan struct{})
	p := startPeerChannels(t, host, user, func(newChannel ssh.NewChannel) {
		defer close(done)
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
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("browser close leaked shell")
	}
}

func TestTerminalDialFailures(t *testing.T) {
	for _, mode := range []string{"unknown", "changed", "authentication", "unreachable"} {
		t.Run(mode, func(t *testing.T) {
			f, d, user := dialFixture(t)
			host, _ := newSigner(t)
			p := startPeer(t, host, user)
			var saved Connection
			if mode == "unknown" {
				saved = destination(t, f, p)
			} else {
				saved = approvedTerminal(t, f, d, p)
			}
			switch mode {
			case "changed":
				replacement, _ := newSigner(t)
				p.useKey(replacement, user)
			case "authentication":
				other, _ := newSigner(t)
				p.useKey(host, other)
			case "unreachable":
				_ = p.listener.Close()
			}
			before := p.auths.Load()
			conn := openTerminal(t, f, saved)
			terminalState(t, conn, "connecting")
			status := terminalState(t, conn, "failed")
			want := map[string]string{"unknown": "approve the SSH host fingerprint", "changed": "SSH host key changed", "authentication": "authentication failed", "unreachable": "could not reach SSH host"}[mode]
			if !strings.Contains(status.Message, want) {
				t.Fatalf("failure: %+v", status)
			}
			terminalClosed(t, conn)
			if (mode == "unknown" || mode == "changed") && p.auths.Load() != before {
				t.Fatal("untrusted host reached authentication")
			}
		})
	}
}

func TestTerminalCloseDuringSetup(t *testing.T) {
	f, d, user := dialFixture(t)
	host, _ := newSigner(t)
	started := make(chan struct{})
	done := make(chan struct{})
	p := startPeerChannels(t, host, user, func(newChannel ssh.NewChannel) {
		defer close(done)
		channel, requests, err := newChannel.Accept()
		if err != nil {
			return
		}
		defer channel.Close()
		for range requests {
			close(started)
			// Leave the PTY request unanswered until cancellation closes SSH.
		}
	})
	saved := approvedTerminal(t, f, d, p)
	conn := openTerminal(t, f, saved)
	terminalState(t, conn, "connecting")
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("PTY setup not reached")
	}
	_ = conn.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("setup did not cancel on browser disconnect")
	}
}

func TestTerminalNonzeroExit(t *testing.T) {
	f, d, user := dialFixture(t)
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
				_, _ = channel.Write([]byte("last output"))
				_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{7}))
				return
			}
		}
	})
	saved := approvedTerminal(t, f, d, p)
	conn := openTerminal(t, f, saved)
	terminalState(t, conn, "connecting")
	terminalState(t, conn, "connected")
	_, data, err := conn.ReadMessage()
	if err != nil || string(data) != "last output" {
		t.Fatalf("tail: %s %v", data, err)
	}
	terminalState(t, conn, "disconnected")
	terminalClosed(t, conn)
}
