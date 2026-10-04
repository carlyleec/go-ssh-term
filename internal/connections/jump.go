package connections

import (
	"context"
	"errors"
	"net"
	"sync"

	"github.com/carlyleec/go-ssh-term/internal/database/sqlite/queries"
	"golang.org/x/crypto/ssh"
)

// Every forwarded transport owns its bastion. Closing the outer transport first
// interrupts blocked channel writes; no other terminal shares this client.
type jumpTransport struct {
	net.Conn
	bastion *ssh.Client
	once    sync.Once
	err     error
}

func (c *jumpTransport) Close() error {
	c.once.Do(func() {
		c.err = c.bastion.Close()
		_ = c.Conn.Close()
	})
	return c.err
}

func (d *Dialer) transport(ctx context.Context, row queries.SavedConnection, address string, jump *queries.SavedConnection) (net.Conn, error) {
	if !row.JumpConnectionID.Valid {
		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
		if err != nil {
			return nil, failure(502, "could not reach SSH host")
		}
		return conn, nil
	}
	// Revalidate ownership/eligibility after audit persistence without replacing
	// the destination snapshot that this attempt recorded.
	current, err := d.jumpConnection(ctx, row)
	if err != nil {
		return nil, err
	}
	if jump == nil || current == nil || jump.ID != current.ID || jump.AccountID != row.AccountID {
		return nil, hopFailure("bastion", failure(409, "jump connection changed; inspect again"))
	}
	bastion, _, err := d.dialRoute(ctx, *jump, nil)
	if err != nil {
		return nil, hopFailure("bastion", err)
	}
	// DialContext can leave a channel-open goroutine pending. Instead, interrupt
	// the synchronous channel request by closing its dedicated outer connection.
	stop := context.AfterFunc(ctx, func() { _ = bastion.Close() })
	defer stop()
	conn, err := bastion.Dial("tcp", address)
	if err != nil {
		_ = bastion.Close()
		return nil, failure(502, "could not reach SSH target through jump connection")
	}
	if !stop() || ctx.Err() != nil {
		_ = bastion.Close()
		_ = conn.Close()
		return nil, failure(504, "SSH setup timed out or was canceled")
	}
	return &jumpTransport{Conn: conn, bastion: bastion}, nil
}

// A route snapshot is resolved before audit start and used for the whole attempt.
func (d *Dialer) jumpConnection(ctx context.Context, row queries.SavedConnection) (*queries.SavedConnection, error) {
	if !row.JumpConnectionID.Valid {
		return nil, nil
	}
	jump, err := d.owned(ctx, row.AccountID, row.JumpConnectionID.String)
	if err != nil {
		return nil, hopFailure("bastion", err)
	}
	if jump.ID == row.ID || jump.JumpConnectionID.Valid {
		return nil, hopFailure("bastion", failure(409, "select an owned direct jump connection"))
	}
	return &jump, nil
}

// Only fixed safe messages and hop labels cross the API or enter audit records.
func hopFailure(hop string, err error) error {
	if err == nil {
		return nil
	}
	var safe *ConnectionErrorBody
	if !errors.As(err, &safe) {
		return &ConnectionErrorBody{status: 502, Message: "SSH setup failed", Hop: hop}
	}
	result := *safe
	result.Hop = hop
	return &result
}
func targetFailure(err error) error {
	var safe *ConnectionErrorBody
	if errors.As(err, &safe) && safe.Hop == "" {
		return hopFailure("target", err)
	}
	return err
}
func connectionErrorMessage(safe *ConnectionErrorBody) string {
	switch safe.Hop {
	case "bastion":
		return "Bastion: " + safe.Message
	case "target":
		return "Target: " + safe.Message
	default:
		return safe.Message
	}
}
