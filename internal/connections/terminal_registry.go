package connections

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/auth"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

var errLiveTerminal = errors.New("terminal is not available to this login session")

// terminalOwner comes only from verified request context, never a wire message.
type terminalOwner struct {
	accountID string
	session   auth.LoginSession
}

type liveTerminal struct {
	id        string
	owner     terminalOwner
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}
	transport io.Closer
	// Attach the client before PTY setup; publish shell/stdin together once ready.
	client *ssh.Client
	shell  *ssh.Session
	stdin  io.WriteCloser
}

type terminalRegistry struct {
	mu        sync.Mutex
	entries   map[string]*liveTerminal
	revoked   map[string]time.Time
	stopping  bool
	ioTimeout time.Duration
}

func newTerminalRegistry() *terminalRegistry {
	return &terminalRegistry{entries: make(map[string]*liveTerminal), revoked: make(map[string]time.Time), ioTimeout: terminalWriteTimeout}
}

func (r *terminalRegistry) start(parent context.Context, owner terminalOwner, transport io.Closer) (*liveTerminal, error) {
	if owner.accountID == "" || owner.session.ID == "" || !time.Now().Before(owner.session.ExpiresAt) || parent.Err() != nil {
		return nil, errLiveTerminal
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, errLiveTerminal
	}
	ctx, cancel := context.WithDeadline(parent, owner.session.ExpiresAt)
	entry := &liveTerminal{id: id.String(), owner: owner, ctx: ctx, cancel: cancel, done: make(chan struct{}), transport: transport}
	r.mu.Lock()
	r.pruneLocked()
	if r.stopping || !r.revoked[owner.session.ID].IsZero() {
		r.mu.Unlock()
		cancel()
		return nil, errLiveTerminal
	}
	r.entries[entry.id] = entry
	r.mu.Unlock()
	// Expiry and cancellation invalidate pending attempts as well as live shells.
	context.AfterFunc(ctx, func() { r.release(entry) })
	return entry, nil
}

// ownedLocked must be called with mu held. Unknown, expired, and foreign IDs
// deliberately have the same result. A shared account is not a shared login.
func (r *terminalRegistry) ownedLocked(owner terminalOwner, id string) (*liveTerminal, error) {
	entry := r.entries[id]
	if r.stopping || !r.revoked[owner.session.ID].IsZero() || entry == nil || entry.owner.accountID != owner.accountID || entry.owner.session.ID != owner.session.ID ||
		!entry.owner.session.ExpiresAt.Equal(owner.session.ExpiresAt) || entry.ctx.Err() != nil || !time.Now().Before(entry.owner.session.ExpiresAt) {
		return nil, errLiveTerminal
	}
	return entry, nil
}

func (r *terminalRegistry) attach(owner terminalOwner, id string, client *ssh.Client) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, err := r.ownedLocked(owner, id)
	if err != nil {
		return err
	}
	if entry.client != nil || client == nil {
		return errLiveTerminal
	}
	entry.client = client
	return nil
}

func (r *terminalRegistry) publish(owner terminalOwner, id string, client *ssh.Client, shell *ssh.Session, stdin io.WriteCloser) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, err := r.ownedLocked(owner, id)
	if err != nil {
		return err
	}
	if entry.shell != nil || entry.client != client || client == nil || shell == nil || stdin == nil {
		return errLiveTerminal
	}
	entry.client, entry.shell, entry.stdin = client, shell, stdin
	return nil
}

func (r *terminalRegistry) input(owner terminalOwner, id string, data []byte) error {
	r.mu.Lock()
	entry, err := r.ownedLocked(owner, id)
	var stdin io.WriteCloser
	if err == nil {
		stdin = entry.stdin
	}
	r.mu.Unlock()
	if err != nil || stdin == nil {
		return errLiveTerminal
	}
	timer := time.AfterFunc(r.ioTimeout, func() { r.release(entry) })
	defer timer.Stop()
	// Network I/O never holds the registry lock; closing the client interrupts
	// an operation admitted just before expiry or removal.
	n, err := stdin.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return err
}

func (r *terminalRegistry) resize(owner terminalOwner, id string, rows, cols int) error {
	r.mu.Lock()
	entry, err := r.ownedLocked(owner, id)
	var shell *ssh.Session
	if err == nil {
		shell = entry.shell
	}
	r.mu.Unlock()
	if err != nil || shell == nil {
		return errLiveTerminal
	}
	timer := time.AfterFunc(r.ioTimeout, func() { r.release(entry) })
	defer timer.Stop()
	return shell.WindowChange(rows, cols)
}

func (r *terminalRegistry) close(owner terminalOwner, id string) error {
	r.mu.Lock()
	entry, err := r.ownedLocked(owner, id)
	if err == nil {
		entry.cancel()
	}
	r.mu.Unlock()
	if err != nil {
		return err
	}
	r.release(entry)
	return nil
}

// release is internal cleanup for this exact attempt, including after expiry.
// It is not a caller-authorized operation and cannot remove a replacement entry.
func (r *terminalRegistry) release(entry *liveTerminal) {
	r.mu.Lock()
	if r.entries[entry.id] != entry {
		r.mu.Unlock()
		return
	}
	entry.cancel()
	client, transport := entry.client, entry.transport
	r.mu.Unlock()
	if transport != nil {
		_ = transport.Close()
	}
	if client != nil {
		_ = client.Close()
	}
}

// complete runs only after the owning handler has joined its workers. Closing
// entries stay discoverable until then so logout and shutdown can await cleanup.
func (r *terminalRegistry) complete(entry *liveTerminal) {
	r.release(entry)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries[entry.id] == entry {
		delete(r.entries, entry.id)
		close(entry.done)
	}
}

func (r *terminalRegistry) pruneLocked() {
	for id, deadline := range r.revoked {
		if !time.Now().Before(deadline) {
			delete(r.revoked, id)
		}
	}
}

// InvalidateSession is the synchronous Access.OnLogout callback. Retain a
// revocation until the original expiry to reject requests admitted before logout.
func (d *Dialer) InvalidateSession(login auth.LoginSession) {
	r := d.terminals
	r.mu.Lock()
	r.pruneLocked()
	if login.ID != "" && time.Now().Before(login.ExpiresAt) && login.ExpiresAt.After(r.revoked[login.ID]) {
		r.revoked[login.ID] = login.ExpiresAt
	}
	var closing []*liveTerminal
	for _, entry := range r.entries {
		if entry.owner.session.ID == login.ID {
			closing = append(closing, entry)
		}
	}
	r.mu.Unlock()
	for _, entry := range closing {
		r.release(entry)
	}
	for _, entry := range closing {
		<-entry.done
	}
}

// Shutdown permanently stops admission, closes transports, and waits for worker
// cleanup within the server's shutdown budget. Repeated calls are safe.
func (d *Dialer) Shutdown(ctx context.Context) error {
	r := d.terminals
	r.mu.Lock()
	r.stopping = true
	var closing []*liveTerminal
	for _, entry := range r.entries {
		closing = append(closing, entry)
	}
	r.mu.Unlock()
	for _, entry := range closing {
		r.release(entry)
	}
	for _, entry := range closing {
		select {
		case <-entry.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
