package connections

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

const (
	terminalProtocol       = "ssh-terminal.v1"
	terminalDataLimit      = 32 * 1024
	terminalControlLimit   = 1024
	terminalDimensionLimit = 1000
	terminalWriteTimeout   = 5 * time.Second
)

var errTerminalMessage = errors.New("invalid terminal message")

type terminalResize struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

type terminalStatus struct {
	Type    string `json:"type"`
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
}

// terminalSocket has one reader and one writer. Callers serialize output and
// status writes; close may run concurrently. No terminal data is logged.
type terminalSocket struct{ conn *websocket.Conn }

func newTerminalSocket(conn *websocket.Conn) *terminalSocket {
	conn.SetReadLimit(terminalDataLimit)
	return &terminalSocket{conn: conn}
}

// read returns either raw input bytes or a resize. Message limits apply across
// continuation frames, before an entire payload is buffered.
func (s *terminalSocket) read() ([]byte, *terminalResize, error) {
	kind, reader, err := s.conn.NextReader()
	if err != nil {
		return nil, nil, err
	}
	limit := terminalDataLimit
	if kind == websocket.TextMessage {
		limit = terminalControlLimit
	}
	payload, err := io.ReadAll(io.LimitReader(reader, int64(limit+1)))
	if err != nil {
		return nil, nil, err
	}
	if len(payload) > limit {
		s.close(websocket.CloseMessageTooBig, "terminal message too large")
		return nil, nil, errTerminalMessage
	}
	if kind == websocket.BinaryMessage {
		return payload, nil, nil
	}
	var resize terminalResize
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&resize)
	var extra any
	if !utf8.Valid(payload) || err != nil || decoder.Decode(&extra) != io.EOF ||
		resize.Type != "resize" || resize.Cols < 1 || resize.Cols > terminalDimensionLimit ||
		resize.Rows < 1 || resize.Rows > terminalDimensionLimit {
		s.close(websocket.ClosePolicyViolation, "invalid terminal message")
		return nil, nil, errTerminalMessage
	}
	return nil, &resize, nil
}

func (s *terminalSocket) writeData(data []byte) error {
	if len(data) > terminalDataLimit {
		return errTerminalMessage
	}
	return s.write(websocket.BinaryMessage, data)
}

// Messages must be safe user-facing text, never raw SSH or storage errors.
func (s *terminalSocket) writeStatus(state, message string) error {
	switch state {
	case "connecting", "connected", "failed", "disconnected":
	default:
		return errTerminalMessage
	}
	if !utf8.ValidString(message) || len(message) > 256 {
		return errTerminalMessage
	}
	payload, err := json.Marshal(terminalStatus{Type: "status", State: state, Message: message})
	if err != nil {
		return err
	}
	if len(payload) > terminalControlLimit {
		return errTerminalMessage
	}
	return s.write(websocket.TextMessage, payload)
}

func (s *terminalSocket) write(kind int, payload []byte) error {
	if err := s.conn.SetWriteDeadline(time.Now().Add(terminalWriteTimeout)); err != nil {
		return err
	}
	return s.conn.WriteMessage(kind, payload)
}

func (s *terminalSocket) close(code int, reason string) {
	_ = s.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(terminalWriteTimeout))
	_ = s.conn.Close()
}
