package connections

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/database/sqlite"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite/queries"
	"github.com/carlyleec/go-ssh-term/internal/sshkeys"
	"golang.org/x/crypto/ssh"
)

const SSHSetupTimeout = 10 * time.Second

var errProbeComplete = errors.New("host key inspection complete")

// Dialer opens direct or single-jump SSH connections. Callers supply verified account identity
// and own the returned client's lifetime; no live handles are persisted here.
type Dialer struct {
	db           *sql.DB
	q            *queries.Queries
	encryption   *sshkeys.Encryption
	timeout      time.Duration
	terminals    *terminalRegistry
	pingInterval time.Duration
	pongWait     time.Duration
}

func NewDialer(db *sql.DB, encryption *sshkeys.Encryption) *Dialer {
	return &Dialer{db: db, q: queries.New(db), encryption: encryption, timeout: SSHSetupTimeout, terminals: newTerminalRegistry(), pingInterval: 15 * time.Second, pongWait: 45 * time.Second}
}

func (d *Dialer) owned(ctx context.Context, accountID, id string) (queries.SavedConnection, error) {
	if !canonicalID(accountID) || !canonicalID(id) {
		return queries.SavedConnection{}, failure(404, "connection not found")
	}
	ctx, cancel := sqlite.WorkContext(ctx)
	defer cancel()
	row, err := d.q.GetOwnedConnection(ctx, queries.GetOwnedConnectionParams{ID: id, AccountID: accountID})
	if errors.Is(err, sql.ErrNoRows) {
		return row, failure(404, "connection not found")
	}
	if err != nil {
		return row, failure(503, "could not load connection; try again")
	}
	host, ok := NormalizeHost(row.Host)
	if !ok || host != row.Host {
		return row, failure(400, "connection host is not normalized")
	}
	return row, nil
}
func (d *Dialer) trusted(ctx context.Context, row queries.SavedConnection) ([]byte, error) {
	ctx, cancel := sqlite.WorkContext(ctx)
	defer cancel()
	trust, err := d.q.GetHostTrust(ctx, queries.GetHostTrustParams{AccountID: row.AccountID, Host: row.Host, Port: row.Port})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, failure(503, "could not load host trust; try again")
	}
	if _, err := ssh.ParsePublicKey(trust.PublicKey); err != nil {
		return nil, failure(503, "stored host trust is invalid; explicitly reset trust")
	}
	return trust.PublicKey, nil
}
func hostAlgorithms(trusted []byte) []string {
	algorithms := []string{ssh.KeyAlgoED25519, ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521, ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256}
	if key, err := ssh.ParsePublicKey(trusted); err == nil {
		preferred := []string{key.Type()}
		if key.Type() == ssh.KeyAlgoRSA {
			preferred = []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256}
		}
		for _, p := range preferred {
			for _, a := range algorithms {
				if p == a {
					algorithms = append([]string{p}, algorithms...)
					break
				}
			}
		}
	}
	return algorithms
}

// exchange bounds TCP setup and the entire SSH handshake. Context cancellation
// closes the socket even while the peer is silent. Successful clients have their
// setup deadline removed before ownership is transferred to the caller.
func (d *Dialer) exchange(ctx context.Context, row queries.SavedConnection, config *ssh.ClientConfig) (*ssh.Client, error) {
	jump, err := d.jumpConnection(ctx, row)
	if err != nil {
		return nil, err
	}
	return d.exchangeRoute(ctx, row, config, jump)
}
func (d *Dialer) exchangeRoute(ctx context.Context, row queries.SavedConnection, config *ssh.ClientConfig, jump *queries.SavedConnection) (*ssh.Client, error) {
	address := net.JoinHostPort(row.Host, strconv.FormatInt(row.Port, 10))
	conn, err := d.transport(ctx, row, address, jump)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	// Forwarded SSH channels have no deadline support. The setup context closes
	// the entire owning transport instead, bounding both channel I/O and handshake.
	if !row.JumpConnectionID.Valid {
		if err = conn.SetDeadline(deadline); err != nil {
			_ = conn.Close()
			return nil, failure(502, "could not configure SSH connection")
		}
	}
	clientConn, chans, requests, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if !stop() || ctx.Err() != nil {
		_ = clientConn.Close()
		return nil, failure(504, "SSH setup timed out or was canceled")
	}
	if !row.JumpConnectionID.Valid {
		if err = conn.SetDeadline(time.Time{}); err != nil {
			_ = clientConn.Close()
			return nil, failure(502, "could not configure SSH connection")
		}
	}
	return ssh.NewClient(clientConn, chans, requests), nil
}
func setupError(ctx context.Context, err error) error {
	var networkError net.Error
	if ctx.Err() != nil || errors.As(err, &networkError) && networkError.Timeout() {
		timeout := failure(504, "SSH setup timed out or was canceled")
		var original *ConnectionErrorBody
		if errors.As(err, &original) {
			timeout.Hop = original.Hop
		}
		return timeout
	}
	var safe *ConnectionErrorBody
	if errors.As(err, &safe) {
		return safe
	}
	return failure(502, "SSH handshake or authentication failed")
}
func (d *Dialer) probe(ctx context.Context, row queries.SavedConnection, trusted []byte) (ssh.PublicKey, error) {
	var observed ssh.PublicKey
	_, err := d.exchange(ctx, row, &ssh.ClientConfig{User: row.Username, HostKeyAlgorithms: hostAlgorithms(trusted), HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
		observed = key
		return errProbeComplete
	}})
	if errors.Is(err, errProbeComplete) && observed != nil && ctx.Err() == nil {
		return observed, nil
	}
	return nil, setupError(ctx, err)
}

// Dial verifies trust during key exchange before loading a private signer or
// sending user authentication. A fresh connection always re-verifies its host.
func (d *Dialer) Dial(ctx context.Context, accountID, id string) (*ssh.Client, Connection, error) {
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	row, err := d.owned(ctx, accountID, id)
	if err != nil {
		return nil, Connection{}, err
	}
	return d.dialConnection(ctx, row)
}

func (d *Dialer) dialConnection(ctx context.Context, row queries.SavedConnection) (*ssh.Client, Connection, error) {
	jump, err := d.jumpConnection(ctx, row)
	if err != nil {
		return nil, Connection{}, err
	}
	return d.dialRoute(ctx, row, jump)
}
func (d *Dialer) dialRoute(ctx context.Context, row queries.SavedConnection, jump *queries.SavedConnection) (client *ssh.Client, snapshot Connection, err error) {
	defer func() { err = targetFailure(err) }()
	trusted, err := d.trusted(ctx, row)
	if err != nil {
		return nil, Connection{}, err
	}
	if trusted == nil {
		return nil, Connection{}, failure(409, "approve the SSH host fingerprint before connecting")
	}
	client, err = d.exchangeRoute(ctx, row, &ssh.ClientConfig{User: row.Username, HostKeyAlgorithms: hostAlgorithms(trusted), HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
		if !bytes.Equal(trusted, key.Marshal()) {
			return failure(409, "SSH host key changed; connection blocked until explicit trust reset")
		}
		// A reset during TCP setup must not authorize this new handshake.
		current, err := d.trusted(ctx, row)
		if err != nil {
			return err
		}
		if !bytes.Equal(current, trusted) {
			return failure(409, "host trust changed; inspect the host again")
		}
		return nil
	}, Auth: []ssh.AuthMethod{ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
		signer, err := d.encryption.Signer(ctx, d.db, row.AccountID, row.SshKeyID)
		if err != nil {
			return nil, failure(503, "could not load SSH authentication key")
		}
		return []ssh.Signer{signer}, nil
	})}}, jump)
	if err != nil {
		return nil, Connection{}, setupError(ctx, err)
	}
	return client, metadata(row), nil
}
