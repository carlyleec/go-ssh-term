package sshkeys

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/carlyleec/go-ssh-term/internal/database/testdb"
)

const testAccount = "11111111-1111-4111-8111-111111111111"
const testKeyID = "22222222-2222-4222-8222-222222222222"

func TestEncryptionKeyPersists(t *testing.T) {
	db, _ := testdb.New(t)
	path := filepath.Join(privateDirectory(t), "application.key")
	if _, err := OpenEncryption(t.Context(), db, path); err != nil {
		t.Fatal(err)
	}
	key, err := os.ReadFile(path)
	if err != nil || len(key) != 32 || bytes.Equal(key, make([]byte, 32)) {
		t.Fatal("did not persist a random 32-byte key")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("key permissions are not 0600")
	}
	plain := []byte("private-key-test-material")
	putEncryptedRecord(t, db, fixtureCiphertext(t, key, plain))
	for range 2 {
		e, err := OpenEncryption(t.Context(), db, path)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(key, data) {
			t.Fatal("startup replaced encryption material")
		}
		got, err := e.decrypt(testKeyID, testAccount, fixtureCiphertext(t, key, plain))
		if err != nil || !bytes.Equal(got, plain) {
			t.Fatal("stored key is not decryptable after restart")
		}
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatal("temporary key file left after initialization")
	}
}

func TestEncryptionStartupFailures(t *testing.T) {
	for _, name := range []string{"missing", "wrong key", "truncated key", "oversized key", "public permissions", "tampered record", "unsupported version", "short ciphertext", "wrong account", "wrong record"} {
		t.Run(name, func(t *testing.T) {
			db, _ := testdb.New(t)
			path := filepath.Join(privateDirectory(t), "application.key")
			key := bytes.Repeat([]byte{42}, 32)
			blob := fixtureCiphertext(t, key, []byte("private-key-test-material"))
			if name == "tampered record" {
				blob[len(blob)-1] ^= 1
			}
			if name == "unsupported version" {
				blob[0] = 2
			}
			if name == "short ciphertext" {
				blob = []byte{1}
			}
			putEncryptedRecord(t, db, blob)
			if name == "wrong account" {
				if _, err := db.ExecContext(t.Context(), `INSERT INTO accounts SELECT '33333333-3333-4333-8333-333333333333', display_name, rp_id, X'02', created_at FROM accounts;
					UPDATE ssh_keys SET account_id = '33333333-3333-4333-8333-333333333333'`); err != nil {
					t.Fatal(err)
				}
			}
			if name == "wrong record" {
				if _, err := db.ExecContext(t.Context(), "UPDATE ssh_keys SET id = '33333333-3333-4333-8333-333333333333'"); err != nil {
					t.Fatal(err)
				}
			}
			switch name {
			case "wrong key":
				key[0] ^= 1
			case "truncated key":
				key = key[:31]
			case "oversized key":
				key = append(key, 0)
			}
			if name != "missing" {
				if err := os.WriteFile(path, key, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if name == "public permissions" {
				if err := os.Chmod(path, 0644); err != nil {
					t.Fatal(err)
				}
			}
			_, err := OpenEncryption(t.Context(), db, path)
			if err == nil || strings.Contains(err.Error(), "private-key-test-material") {
				t.Fatal("startup did not fail safely")
			}
			got, readErr := os.ReadFile(path)
			if name == "missing" {
				if !os.IsNotExist(readErr) {
					t.Fatal("missing encryption material was replaced")
				}
			} else if readErr != nil || !bytes.Equal(got, key) {
				t.Fatal("failed startup modified existing encryption material")
			}
		})
	}
}

func TestEncryptionStorageFailures(t *testing.T) {
	for _, name := range []string{"missing directory", "public directory", "unwritable directory", "unreadable key", "symlink", "directory key", "malformed empty-database key", "closed database", "canceled"} {
		t.Run(name, func(t *testing.T) {
			if os.Geteuid() == 0 && (name == "unwritable directory" || name == "unreadable key") {
				t.Skip("requires a non-root process")
			}
			db, _ := testdb.New(t)
			dir := privateDirectory(t)
			path := filepath.Join(dir, "application.key")
			ctx := t.Context()
			switch name {
			case "missing directory":
				path = filepath.Join(dir, "missing", "application.key")
			case "public directory", "unwritable directory":
				mode := os.FileMode(0755)
				if name == "unwritable directory" {
					mode = 0500
				}
				if err := os.Chmod(dir, mode); err != nil {
					t.Fatal(err)
				}
				defer os.Chmod(dir, 0700)
			case "unreadable key":
				if err := os.WriteFile(path, make([]byte, 32), 0000); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				target := filepath.Join(t.TempDir(), "target")
				if err := os.WriteFile(target, make([]byte, 32), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			case "directory key":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "malformed empty-database key":
				if err := os.WriteFile(path, nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "closed database":
				db.Close()
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if _, err := OpenEncryption(ctx, db, path); err == nil {
				t.Fatal("unsafe initialization succeeded")
			}
			if name == "closed database" || name == "canceled" {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("database failure created encryption material")
				}
			}
		})
	}
}

func TestConcurrentEncryptionInitialization(t *testing.T) {
	db, _ := testdb.New(t)
	path := filepath.Join(privateDirectory(t), "application.key")
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			if _, err := OpenEncryption(t.Context(), db, path); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if key, err := os.ReadFile(path); err != nil || len(key) != 32 {
		t.Fatal("concurrent initialization did not persist a complete key")
	}
}

// Construct the documented on-disk format using the standard library directly.
func fixtureCiphertext(t *testing.T, key, plain []byte) []byte {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		t.Fatal(err)
	}
	aad := []byte("go-ssh-term:ssh-key:v1\x00" + testAccount + "\x00" + testKeyID)
	return append([]byte{1}, aead.Seal(nil, nil, plain, aad)...)
}

func putEncryptedRecord(t *testing.T, db *sql.DB, blob []byte) {
	t.Helper()
	if _, err := db.ExecContext(t.Context(), `INSERT INTO accounts VALUES (?, 'Test', 'localhost', X'01', 1)`, testAccount); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `INSERT INTO ssh_keys VALUES (?, ?, 'Test key', 'SHA256:test', ?, 1)`, testKeyID, testAccount, blob); err != nil {
		t.Fatal(err)
	}
}

func privateDirectory(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}
