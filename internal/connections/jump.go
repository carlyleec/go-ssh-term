package connections

import (
	"context"
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

func (d *Dialer) transport(ctx context.Context, row queries.SavedConnection, address string) (net.Conn, error) {
	if !row.JumpConnectionID.Valid {
		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
		if err != nil {
			return nil, failure(502, "could not reach SSH host")
		}
		return conn, nil
	}
	jump, err := d.owned(ctx, row.AccountID, row.JumpConnectionID.String)
	if err != nil {
		return nil, err
	}
	if jump.ID == row.ID || jump.JumpConnectionID.Valid {
		return nil, failure(409, "select an owned direct jump connection")
	}
	bastion, _, err := d.dialConnection(ctx, jump)
	if err != nil {
		return nil, err
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
