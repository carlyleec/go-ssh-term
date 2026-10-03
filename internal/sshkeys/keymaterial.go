package sshkeys

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/carlyleec/go-ssh-term/internal/config"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite/queries"
)

// OpenEncryption runs before serving requests. Only an empty SSH key table
// permits first-time key creation; database failures never count as emptiness.
func OpenEncryption(ctx context.Context, db *sql.DB, path string) (*Encryption, error) {
	if err := config.ValidateEncryptionKeyPath(path); err != nil {
		return nil, err
	}
	parent, err := os.Stat(filepath.Dir(path))
	if err != nil || !parent.IsDir() || parent.Mode().Perm()&0077 != 0 {
		return nil, errors.New("encryption-key directory must exist with owner-only permissions; check ENCRYPTION_KEY_PATH and run storage-init")
	}
	ctx, cancel := sqlite.WorkContext(ctx)
	defer cancel()
	records, err := queries.New(db).ListEncryptedSSHKeys(ctx)
	if err != nil {
		return nil, errors.New("cannot check stored SSH keys; application encryption key was not created")
	}
	key, err := readEncryptionKey(path)
	if errors.Is(err, os.ErrNotExist) {
		if len(records) != 0 {
			return nil, errors.New("application encryption key is missing but SSH keys exist; restore the encryption-key volume from backup")
		}
		if ctx.Err() != nil {
			return nil, errors.New("application encryption initialization was canceled")
		}
		key, err = createEncryptionKey(path)
	}
	if err != nil {
		return nil, err
	}
	defer clear(key)
	encryption, err := newEncryption(key)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if ctx.Err() != nil {
			return nil, errors.New("stored SSH key verification timed out or was canceled")
		}
		plain, err := encryption.decrypt(record.ID, record.AccountID, record.EncryptedPrivateKey)
		clear(plain)
		if err != nil {
			return nil, err
		}
	}
	return encryption, nil
}

func readEncryptionKey(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, os.ErrNotExist
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() != 32 {
		return nil, errors.New("application encryption key must be a regular, owner-only 32-byte file; restore a valid key or check ENCRYPTION_KEY_PATH")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("cannot read application encryption key; check file ownership and permissions")
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.New("application encryption key changed while opening it; retry startup")
	}
	key, err := io.ReadAll(io.LimitReader(file, 33))
	if err != nil || len(key) != 32 {
		clear(key)
		return nil, errors.New("cannot read a complete application encryption key; restore the encryption-key volume")
	}
	return key, nil
}

func createEncryptionKey(path string) ([]byte, error) {
	key := make([]byte, 32)
	defer clear(key)
	if _, err := rand.Read(key); err != nil {
		return nil, errors.New("cannot generate application encryption key")
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".encryption-key-*")
	if err != nil {
		return nil, errors.New("cannot create application encryption key; check directory ownership and permissions")
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := file.Write(key); err != nil {
		return nil, errors.New("cannot write application encryption key")
	}
	if err := file.Sync(); err != nil {
		return nil, errors.New("cannot persist application encryption key")
	}
	if err := file.Close(); err != nil {
		return nil, errors.New("cannot close application encryption key")
	}
	// Publish only a fully written file. Unlike rename, link never overwrites
	// an existing key if another initializer reaches this point first.
	if err := os.Link(file.Name(), path); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, errors.New("cannot publish application encryption key; check the persistent filesystem")
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return nil, errors.New("cannot open encryption-key directory for persistence")
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return nil, errors.New("cannot persist encryption-key directory")
	}
	return readEncryptionKey(path)
}
