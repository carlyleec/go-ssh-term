package connections

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func TestTerminalUpgradeGuards(t *testing.T) {
	f := setup(t)
	saved := result(t, f.request("POST", "/api/connections", f.fields(), 0), 201)
	server := httptest.NewServer(f.handler)
	defer server.Close()
	for _, tc := range []struct {
		name, origin, id, protocol string
		owner, status              int
	}{
		{"anonymous", origin, saved.ID, terminalProtocol, -1, 401},
		{"missing origin", "", saved.ID, terminalProtocol, 0, 403},
		{"wrong origin", "http://localhost:8080", saved.ID, terminalProtocol, 0, 403},
		{"null origin", "null", saved.ID, terminalProtocol, 0, 403},
		{"other owner", origin, saved.ID, terminalProtocol, 1, 404},
		{"missing connection", origin, uuid.NewString(), terminalProtocol, 0, 404},
		{"invalid id", origin, "invalid", terminalProtocol, 0, 404},
		{"missing protocol", origin, saved.ID, "", 0, 400},
		{"wrong protocol", origin, saved.ID, "ssh-terminal.v2", 0, 400},
		{"allowed", origin, saved.ID, terminalProtocol, 0, 101},
	} {
		t.Run(tc.name, func(t *testing.T) {
			headers := http.Header{"Origin": {tc.origin}}
			if tc.owner >= 0 {
				headers.Set("Cookie", f.cookies[tc.owner].String())
			}
			dialer := websocket.Dialer{HandshakeTimeout: time.Second}
			if tc.protocol != "" {
				dialer.Subprotocols = []string{tc.protocol}
			}
			conn, response, err := dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/api/connections/"+tc.id+"/terminal", headers)
			if response == nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != tc.status {
				t.Fatalf("status %d, want %d", response.StatusCode, tc.status)
			}
			if response.Header.Get("Cache-Control") != "no-store" || len(response.Cookies()) != 0 {
				t.Fatal("cache/cookie headers changed")
			}
			if tc.status != 101 {
				if err == nil {
					conn.Close()
					t.Fatal("unauthorized upgrade")
				}
				var body map[string]string
				if json.NewDecoder(response.Body).Decode(&body) != nil || len(body) != 1 || body["error"] == "" {
					t.Fatal("unsafe error body")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if conn.Subprotocol() != terminalProtocol {
				t.Fatal("protocol not negotiated")
			}
			_ = conn.SetReadDeadline(time.Now().Add(time.Second))
			kind, data, err := conn.ReadMessage()
			if err != nil || kind != websocket.TextMessage || string(data) != `{"type":"status","state":"failed","message":"Terminal sessions are not available yet."}` {
				t.Fatalf("status: %s %v", data, err)
			}
			_, _, err = conn.ReadMessage()
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				t.Fatalf("close: %v", err)
			}
		})
	}
	// Plain GET and malformed upgrades use the same origin/session boundary.
	for _, tc := range []struct {
		owner, status int
		requestOrigin string
	}{{0, 403, ""}, {-1, 401, origin}, {0, 400, origin}} {
		r := httptest.NewRequest("GET", "/api/connections/"+saved.ID+"/terminal", nil)
		r.Header.Set("Origin", tc.requestOrigin)
		r.Header.Set("Sec-WebSocket-Protocol", terminalProtocol)
		if tc.owner >= 0 {
			r.AddCookie(f.cookies[tc.owner])
		}
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, r)
		expect(t, w, tc.status)
	}
}

func TestTerminalExpiredSessionAndStorageFailure(t *testing.T) {
	for _, mode := range []string{"expired", "logout", "account deleted", "connection storage", "session storage"} {
		t.Run(mode, func(t *testing.T) {
			f := setup(t)
			saved := result(t, f.request("POST", "/api/connections", f.fields(), 0), 201)
			want := 401
			switch mode {
			case "expired":
				ctx, err := f.sessions.Load(t.Context(), f.cookies[0].Value)
				if err != nil {
					t.Fatal(err)
				}
				f.sessions.SetDeadline(ctx, time.Now().Add(-time.Minute))
				if _, _, err := f.sessions.Commit(ctx); err != nil {
					t.Fatal(err)
				}
			case "logout":
				if err := f.sessions.Store.Delete(f.cookies[0].Value); err != nil {
					t.Fatal(err)
				}
			case "account deleted":
				if _, err := f.db.Exec("DELETE FROM accounts WHERE id = ?", f.owners[0]); err != nil {
					t.Fatal(err)
				}
			case "connection storage":
				want = 503
				if _, err := f.db.Exec("DROP TABLE saved_connections"); err != nil {
					t.Fatal(err)
				}
			case "session storage":
				want = 503
				if err := f.db.Close(); err != nil {
					t.Fatal(err)
				}
			}
			r := httptest.NewRequest("GET", "/api/connections/"+saved.ID+"/terminal", nil)
			r.AddCookie(f.cookies[0])
			r.Header.Set("Origin", origin)
			r.Header.Set("Upgrade", "websocket")
			w := httptest.NewRecorder()
			f.handler.ServeHTTP(w, r)
			expect(t, w, want)
			if strings.Contains(w.Body.String(), "SQL") || strings.Contains(w.Body.String(), "saved_connections") {
				t.Fatal("storage details exposed")
			}
		})
	}
}

// The loopback peer exercises transport helpers without attaching an SSH shell.
func terminalPeer(t *testing.T, serve func(*terminalSocket)) *websocket.Conn {
	t.Helper()
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		serve(newTerminalSocket(conn))
	}))
	dialer := websocket.Dialer{WriteBufferSize: 128, HandshakeTimeout: time.Second}
	conn, _, err := dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	t.Cleanup(func() { conn.Close(); server.Close(); <-done })
	return conn
}

func TestTerminalProtocolRoundTrip(t *testing.T) {
	payload := bytes.Repeat([]byte{0, 3, 27, 0xff, 0xe2, 0x82, 0xac, 13}, terminalDataLimit/8)
	conn := terminalPeer(t, func(socket *terminalSocket) {
		input, resize, err := socket.read()
		if err != nil || resize != nil || !bytes.Equal(input, payload) {
			t.Errorf("input mismatch: %v", err)
			return
		}
		if err := socket.writeData(input); err != nil {
			t.Error(err)
			return
		}
		input, resize, err = socket.read()
		if err != nil || len(input) != 0 || resize == nil || resize.Cols != 1000 || resize.Rows != 1 {
			t.Errorf("resize: %+v %v", resize, err)
			return
		}
		for _, state := range []string{"connecting", "connected", "failed", "disconnected"} {
			if err := socket.writeStatus(state, ""); err != nil {
				t.Error(err)
				return
			}
		}
	})
	if err := conn.WriteMessage(websocket.BinaryMessage, payload); err != nil {
		t.Fatal(err)
	}
	kind, output, err := conn.ReadMessage()
	if err != nil || kind != websocket.BinaryMessage || !bytes.Equal(payload, output) {
		t.Fatalf("output mismatch: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"resize","cols":1000,"rows":1}`)); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"connecting", "connected", "failed", "disconnected"} {
		kind, data, err := conn.ReadMessage()
		if err != nil || kind != websocket.TextMessage || string(data) != `{"type":"status","state":"`+state+`"}` {
			t.Fatalf("status: %s %v", data, err)
		}
	}
}

func TestTerminalProtocolRejectsInvalidMessages(t *testing.T) {
	for _, payload := range []string{
		`{`, `null`, `[]`, `{}`, `{"type":"status","state":"connected"}`,
		`{"type":"resize","cols":0,"rows":24}`, `{"type":"resize","cols":80,"rows":1001}`,
		`{"type":"resize","cols":-1,"rows":24}`, `{"type":"resize","cols":80.5,"rows":24}`,
		`{"type":"resize","cols":"80","rows":24}`, `{"type":"resize","cols":80}`,
		`{"type":"resize","cols":80,"rows":24,"extra":true}`,
		`{"type":"resize","cols":80,"rows":24} {}`, "\xff",
	} {
		t.Run(payload, func(t *testing.T) {
			conn := terminalPeer(t, func(socket *terminalSocket) {
				if _, _, err := socket.read(); err == nil {
					t.Error("invalid message accepted")
				}
			})
			if err := conn.WriteMessage(websocket.TextMessage, []byte(payload)); err != nil {
				t.Fatal(err)
			}
			_, _, err := conn.ReadMessage()
			if !websocket.IsCloseError(err, websocket.ClosePolicyViolation) {
				t.Fatalf("close: %v", err)
			}
		})
	}
}

func TestTerminalMessageLimits(t *testing.T) {
	for _, kind := range []int{websocket.TextMessage, websocket.BinaryMessage} {
		for _, fragmented := range []bool{false, true} {
			conn := terminalPeer(t, func(socket *terminalSocket) {
				if _, _, err := socket.read(); err == nil {
					t.Error("oversized message accepted")
				}
			})
			limit := terminalDataLimit
			if kind == websocket.TextMessage {
				limit = terminalControlLimit
			}
			payload := bytes.Repeat([]byte("x"), limit+1)
			if fragmented {
				// NextWriter flushes continuation frames as its buffer fills.
				writer, err := conn.NextWriter(kind)
				if err != nil {
					t.Fatal(err)
				}
				for _, b := range payload {
					if _, err := writer.Write([]byte{b}); err != nil {
						break
					}
				}
				_ = writer.Close()
			} else {
				_ = conn.WriteMessage(kind, payload)
			}
			_, _, err := conn.ReadMessage()
			if !websocket.IsCloseError(err, websocket.CloseMessageTooBig) {
				t.Fatalf("kind %d fragmented %v: %v", kind, fragmented, err)
			}
		}
	}
}

func TestTerminalOutgoingLimits(t *testing.T) {
	// Invalid writes fail before touching a connection.
	socket := &terminalSocket{}
	if socket.writeData(make([]byte, terminalDataLimit+1)) == nil {
		t.Fatal("unbounded output")
	}
	for _, tc := range [][2]string{{"bogus", ""}, {"failed", strings.Repeat("x", 257)}, {"failed", "\xff"}, {"failed", strings.Repeat("\x00", 256)}} {
		if socket.writeStatus(tc[0], tc[1]) == nil {
			t.Fatal("invalid status accepted")
		}
	}
}

func TestTerminalControlLimitBoundary(t *testing.T) {
	conn := terminalPeer(t, func(socket *terminalSocket) {
		_, resize, err := socket.read()
		if err != nil || resize == nil {
			t.Errorf("boundary resize: %v", err)
			return
		}
		socket.close(websocket.CloseNormalClosure, "")
	})
	message := `{"type":"resize","cols":80,"rows":24}`
	message += strings.Repeat(" ", terminalControlLimit-len(message))
	if err := conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
		t.Fatal(err)
	}
	_, _, err := conn.ReadMessage()
	if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
		t.Fatal(err)
	}
}
