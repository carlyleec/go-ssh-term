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
	id     string
	owner  terminalOwner
	ctx    context.Context
	cancel context.CancelFunc
	// Handles are published together under the registry lock after shell setup.
	client *ssh.Client
	shell  *ssh.Session
	stdin  io.WriteCloser
}

type terminalRegistry struct {
	mu      sync.Mutex
	entries map[string]*liveTerminal
}

func newTerminalRegistry() *terminalRegistry {
	return &terminalRegistry{entries: make(map[string]*liveTerminal)}
}

func (r *terminalRegistry) start(parent context.Context, owner terminalOwner) (*liveTerminal, error) {
	if owner.accountID == "" || owner.session.ID == "" || !time.Now().Before(owner.session.ExpiresAt) || parent.Err() != nil {
		return nil, errLiveTerminal
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, errLiveTerminal
	}
	ctx, cancel := context.WithDeadline(parent, owner.session.ExpiresAt)
	entry := &liveTerminal{id: id.String(), owner: owner, ctx: ctx, cancel: cancel}
	r.mu.Lock()
	r.entries[entry.id] = entry
	r.mu.Unlock()
	// Expiry and cancellation remove pending attempts as well as live shells.
	context.AfterFunc(ctx, func() { r.release(entry) })
	return entry, nil
}

// ownedLocked must be called with mu held. Unknown, expired, and foreign IDs
// deliberately have the same result. A shared account is not a shared login.
func (r *terminalRegistry) ownedLocked(owner terminalOwner, id string) (*liveTerminal, error) {
	entry := r.entries[id]
	if entry == nil || entry.owner.accountID != owner.accountID || entry.owner.session.ID != owner.session.ID ||
		!entry.owner.session.ExpiresAt.Equal(owner.session.ExpiresAt) || entry.ctx.Err() != nil || !time.Now().Before(entry.owner.session.ExpiresAt) {
		return nil, errLiveTerminal
	}
	return entry, nil
}

func (r *terminalRegistry) publish(owner terminalOwner, id string, client *ssh.Client, shell *ssh.Session, stdin io.WriteCloser) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, err := r.ownedLocked(owner, id)
	if err != nil {
		return err
	}
	if entry.client != nil || client == nil || shell == nil || stdin == nil {
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
	return shell.WindowChange(rows, cols)
}

func (r *terminalRegistry) close(owner terminalOwner, id string) error {
	r.mu.Lock()
	entry, err := r.ownedLocked(owner, id)
	if err == nil {
		delete(r.entries, id)
	}
	r.mu.Unlock()
	if err != nil {
		return err
	}
	entry.cancel()
	if entry.client != nil {
		_ = entry.client.Close()
	}
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
	delete(r.entries, entry.id)
	r.mu.Unlock()
	entry.cancel()
	if entry.client != nil {
		_ = entry.client.Close()
	}
}
