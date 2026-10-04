package connections

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Run inside the development app container to exercise the gateway's network
// boundary. The fixture creates its own database, account, session, and key file.
func TestImportOpenSSHLab(t *testing.T) {
	if os.Getenv("SSH_IMPORT_LAB") != "1" {
		t.Skip("set SSH_IMPORT_LAB=1 inside the app container with the lab running")
	}
	f, d, _ := dialFixture(t)
	private, err := os.ReadFile("../../demo/keys/demo_ed25519")
	if err != nil {
		t.Fatal(err)
	}
	var upload bytes.Buffer
	writer := multipart.NewWriter(&upload)
	if err := writer.WriteField("name", "Demo import key"); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("private_key", "demo_ed25519")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(private); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/api/keys", &upload)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.Header.Set("Origin", origin)
	r.AddCookie(f.cookies[0])
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	expect(t, w, 201)
	var key string
	if err := f.db.QueryRow("SELECT id FROM ssh_keys WHERE name='Demo import key'").Scan(&key); err != nil {
		t.Fatal(err)
	}
	request := sampleRequest(t, key)
	if !previewResult(t, f, request).CanConfirm || connectionCount(t, f) != 0 {
		t.Fatal("preview invalid or persisted rows")
	}
	// Abandon one preview, then start over and explicitly confirm the same input.
	if !previewResult(t, f, request).CanConfirm {
		t.Fatal("second preview invalid")
	}
	w = f.request("POST", "/api/connections/import/confirm", request, 0)
	expect(t, w, 201)
	var saved ConnectionsBody
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{"bastion": "Bastion host: bastion", "target-1": "Private target: target-1", "target-2": "Private target: target-2"}
	for _, connection := range saved.Connections {
		t.Run(connection.Name, func(t *testing.T) {
			seen, err := d.Inspect(t.Context(), f.owners[0], connection.ID)
			if err != nil {
				t.Fatal(err)
			}
			if seen.State != "trusted" {
				if _, err := d.Approve(t.Context(), f.owners[0], connection.ID, decision(seen)); err != nil {
					t.Fatal(err)
				}
			}
			conn := openTerminal(t, f, connection)
			terminalState(t, conn, "connecting")
			terminalState(t, conn, "connected")
			_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.BinaryMessage, []byte("cat /host-info.txt\n")); err != nil {
				t.Fatal(err)
			}
			var output strings.Builder
			for !strings.Contains(output.String(), expected[connection.Name]) {
				kind, data, err := conn.ReadMessage()
				if err != nil {
					t.Fatalf("no identifying output for %s: %v", connection.Name, err)
				}
				if kind == websocket.BinaryMessage {
					output.Write(data)
				}
				if output.Len() > 65536 {
					t.Fatal("unexpected shell output size")
				}
			}
			t.Logf("imported %s: received %q over terminal WebSocket", connection.Name, expected[connection.Name])
			if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"close"}`)); err != nil {
				t.Fatal(err)
			}
		})
	}
	if len(saved.Connections) != 3 {
		t.Fatal("missing imported hosts")
	}
}
