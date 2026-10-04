package connections

import (
	"encoding/json"
	"net/http"
	"slices"

	"github.com/carlyleec/go-ssh-term/internal/auth"
	"github.com/gorilla/websocket"
)

// RegisterTerminal uses net/http directly: a successful upgrade transfers the
// connection to the WebSocket transport instead of a Huma response serializer.
func (d *Dialer) RegisterTerminal(mux *http.ServeMux, access *auth.Access, origin string) {
	upgrader := websocket.Upgrader{
		Subprotocols:     []string{terminalProtocol},
		HandshakeTimeout: terminalWriteTimeout,
		CheckOrigin:      func(r *http.Request) bool { return r.Header.Get("Origin") == origin },
		Error: func(w http.ResponseWriter, r *http.Request, status int, _ error) {
			terminalHTTPError(w, status, "invalid terminal WebSocket handshake")
		},
	}
	endpoint := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account, _ := auth.AccountFromContext(r.Context())
		_, err := d.owned(r.Context(), account.ID, r.PathValue("id"))
		if err != nil {
			safe := err.(*ConnectionErrorBody)
			terminalHTTPError(w, safe.status, safe.Message)
			return
		}
		if !slices.Contains(websocket.Subprotocols(r), terminalProtocol) {
			terminalHTTPError(w, 400, "request the ssh-terminal.v1 WebSocket subprotocol")
			return
		}
		conn, err := upgrader.Upgrade(w, r, w.Header().Clone())
		if err != nil {
			return
		}
		socket := newTerminalSocket(conn)
		d.runTerminal(r.Context(), socket, account.ID, r.PathValue("id"))
	})
	// Guard every request, including malformed handshakes without Upgrade headers.
	mux.Handle("GET /api/connections/{id}/terminal", auth.RequireOrigin(origin, access.Require(endpoint)))
}

func terminalHTTPError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
