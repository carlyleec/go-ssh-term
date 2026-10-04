package sshkeys

import (
	"context"
	"database/sql"
	"errors"

	"github.com/carlyleec/go-ssh-term/internal/database/sqlite"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite/queries"
	"golang.org/x/crypto/ssh"
)

// Signer loads only the owner's key. Call after host verification; the returned
// signer holds private material in memory until the SSH handshake releases it.
func (e *Encryption) Signer(ctx context.Context, db *sql.DB, accountID, keyID string) (ssh.Signer, error) {
	ctx, cancel := sqlite.WorkContext(ctx)
	defer cancel()
	data, err := queries.New(db).GetOwnedEncryptedSSHKey(ctx, queries.GetOwnedEncryptedSSHKeyParams{ID: keyID, AccountID: accountID})
	if err != nil {
		return nil, errors.New("could not load SSH key")
	}
	plain, err := e.decrypt(keyID, accountID, data)
	defer clear(plain)
	if err != nil {
		return nil, errors.New("could not decrypt SSH key")
	}
	signer, err := ssh.ParsePrivateKey(plain)
	if err != nil {
		return nil, errors.New("could not parse SSH key")
	}
	return signer, nil
}
