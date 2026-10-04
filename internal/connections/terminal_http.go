package connections

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"slices"

	"github.com/carlyleec/go-ssh-term/internal/auth"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite/queries"
	"github.com/gorilla/websocket"
)

// RegisterTerminal uses net/http directly: a successful upgrade transfers the
// connection to the WebSocket transport instead of a Huma response serializer.
func (h *handler) RegisterTerminal(mux *http.ServeMux, access *auth.Access, origin string) {
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
		if !canonicalID(r.PathValue("id")) {
			terminalHTTPError(w, 404, "connection not found")
			return
		}
		ctx, cancel := sqlite.WorkContext(r.Context())
		_, err := h.queries.GetOwnedConnection(ctx, queries.GetOwnedConnectionParams{ID: r.PathValue("id"), AccountID: account.ID})
		cancel()
		if errors.Is(err, sql.ErrNoRows) {
			terminalHTTPError(w, 404, "connection not found")
			return
		}
		if err != nil {
			terminalHTTPError(w, 503, "could not load connection; try again")
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
		defer socket.close(websocket.CloseNormalClosure, "terminal unavailable")
		// The transport is ready, but no SSH shell is attached yet.
		_ = socket.writeStatus("failed", "Terminal sessions are not available yet.")
	})
	// Guard every request, including malformed handshakes without Upgrade headers.
	mux.Handle("GET /api/connections/{id}/terminal", auth.RequireOrigin(origin, access.Require(endpoint)))
}

func terminalHTTPError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
