package sshkeys

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/carlyleec/go-ssh-term/internal/database"
	"github.com/carlyleec/go-ssh-term/internal/database/testdb"
	"golang.org/x/crypto/ssh"
)

func TestEncryptRoundTrip(t *testing.T) {
	e := testEncryption(t)
	upload := testUpload(t)
	fingerprint, err := Validate(upload)
	if err != nil {
		t.Fatal(err)
	}
	for _, plain := range [][]byte{[]byte{0}, upload, append(bytes.Clone(upload), bytes.Repeat([]byte(" "), MaxUploadBytes-len(upload))...)} {
		before := bytes.Clone(plain)
		blob, err := e.encrypt(testKeyID, testAccount, plain)
		if err != nil {
			t.Fatal(err)
		}
		if len(blob) != 1+12+len(plain)+16 || blob[0] != 1 {
			t.Fatal("unexpected encrypted payload format")
		}
		if !bytes.Equal(plain, before) {
			t.Fatal("encryption modified the caller's plaintext")
		}
		stored := bytes.Clone(blob)
		got, err := e.decrypt(testKeyID, testAccount, blob)
		if err != nil || !bytes.Equal(got, before) {
			t.Fatal("plaintext did not round trip")
		}
		if !bytes.Equal(blob, stored) {
			t.Fatal("decryption modified the stored payload")
		}
		if len(plain) > 1 {
			if bytes.Contains(blob, bytes.TrimSpace(upload)) {
				t.Fatal("payload contains plaintext key")
			}
			if gotFingerprint, err := Validate(got); err != nil || gotFingerprint != fingerprint {
				t.Fatal("decrypted SSH key did not retain its fingerprint")
			}
		}
		clear(got)
		if !bytes.Equal(plain, before) || !bytes.Equal(blob, stored) {
			t.Fatal("decrypted buffer aliases an input")
		}
	}
}

func TestEncryptConcurrentNonces(t *testing.T) {
	e := testEncryption(t)
	upload := testUpload(t)
	const count = 32
	results := make(chan []byte, count)
	var wg sync.WaitGroup
	for range count {
		wg.Go(func() {
			blob, err := e.encrypt(testKeyID, testAccount, upload)
			if err != nil {
				t.Error(err)
				return
			}
			plain, err := e.decrypt(testKeyID, testAccount, blob)
			if err != nil || !bytes.Equal(plain, upload) {
				t.Error("concurrent round trip failed")
				return
			}
			clear(plain)
			results <- blob
		})
	}
	wg.Wait()
	close(results)
	seen := make(map[string]bool)
	for blob := range results {
		nonce := string(blob[1:13])
		if seen[nonce] {
			t.Fatal("repeated nonce for the same application key")
		}
		seen[nonce] = true
	}
	if len(seen) != count {
		t.Fatal("missing encryption results")
	}
}

func TestEncryptRejectsInvalidInputs(t *testing.T) {
	e := testEncryption(t)
	for _, tc := range []struct {
		name      string
		e         *Encryption
		id, owner string
		plain     []byte
		want      error
	}{
		{"empty", e, testKeyID, testAccount, nil, ErrInvalid},
		{"oversized", e, testKeyID, testAccount, make([]byte, MaxUploadBytes+1), ErrTooLarge},
		{"nil cipher", nil, testKeyID, testAccount, []byte{1}, errEncrypt},
		{"zero cipher", &Encryption{}, testKeyID, testAccount, []byte{1}, errEncrypt},
		{"missing record", e, "", testAccount, []byte{1}, errEncrypt},
		{"missing account", e, testKeyID, "", []byte{1}, errEncrypt},
		{"context separator", e, testKeyID, testAccount + "\x00extra", []byte{1}, errEncrypt},
		{"noncanonical UUID", e, "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA", testAccount, []byte{1}, errEncrypt},
	} {
		t.Run(tc.name, func(t *testing.T) {
			blob, err := tc.e.encrypt(tc.id, tc.owner, tc.plain)
			if !errors.Is(err, tc.want) || blob != nil {
				t.Fatal("invalid input did not return the expected safe error and nil payload")
			}
		})
	}
	for _, size := range []int{0, 16, 24, 31, 33} {
		if e, err := newEncryption(make([]byte, size)); err == nil || e != nil {
			t.Fatal("non-256-bit application key accepted")
		}
	}
}

func TestDecryptRejectsModifiedEncryptedOutput(t *testing.T) {
	e := testEncryption(t)
	empty := e.aead.Seal([]byte{encryptionVersion}, nil, nil, encryptionContext(testKeyID, testAccount))
	if plain, err := e.decrypt(testKeyID, testAccount, empty); !errors.Is(err, errDecrypt) || plain != nil {
		t.Fatal("authenticated empty plaintext accepted")
	}
	blob, err := e.encrypt(testKeyID, testAccount, testUpload(t))
	if err != nil {
		t.Fatal(err)
	}
	otherKey, err := newEncryption(bytes.Repeat([]byte{43}, 32))
	if err != nil {
		t.Fatal(err)
	}
	for _, offset := range []int{0, 1, 13, len(blob) - 1} {
		changed := bytes.Clone(blob)
		changed[offset] ^= 1
		if plain, err := e.decrypt(testKeyID, testAccount, changed); !errors.Is(err, errDecrypt) || plain != nil {
			t.Fatal("modified payload accepted")
		}
	}
	for _, data := range [][]byte{nil, {1}, blob[:len(blob)-1], append(bytes.Clone(blob), 0), make([]byte, MaxUploadBytes+30)} {
		if plain, err := e.decrypt(testKeyID, testAccount, data); !errors.Is(err, errDecrypt) || plain != nil {
			t.Fatal("invalid payload length accepted")
		}
	}
	for _, tc := range []struct {
		e         *Encryption
		id, owner string
	}{
		{otherKey, testKeyID, testAccount}, {nil, testKeyID, testAccount}, {&Encryption{}, testKeyID, testAccount},
		{e, testAccount, testAccount}, {e, testKeyID, testKeyID}, {e, "", testAccount}, {e, testKeyID, ""},
	} {
		if plain, err := tc.e.decrypt(tc.id, tc.owner, blob); !errors.Is(err, errDecrypt) || plain != nil {
			t.Fatal("wrong cipher or identity accepted")
		}
	}
}

func TestEncryptedUploadSurvivesReopen(t *testing.T) {
	db, dbPath := testdb.New(t)
	keyPath := filepath.Join(privateDirectory(t), "application.key")
	e, err := OpenEncryption(t.Context(), db, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	upload := testUpload(t)
	blob, err := e.encrypt(testKeyID, testAccount, upload)
	if err != nil {
		t.Fatal(err)
	}
	putEncryptedRecord(t, db, blob)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = database.Open(t.Context(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	reopened, err := OpenEncryption(t.Context(), db, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	var stored []byte
	if err := db.QueryRowContext(t.Context(), "SELECT encrypted_private_key FROM ssh_keys WHERE id = ?", testKeyID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	plain, err := reopened.decrypt(testKeyID, testAccount, stored)
	defer clear(plain)
	if err != nil || !bytes.Equal(plain, upload) {
		t.Fatal("encrypted upload did not survive database and encryption-key reopen")
	}
}

func testEncryption(t *testing.T) *Encryption {
	t.Helper()
	e, err := newEncryption(bytes.Repeat([]byte{42}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func testUpload(t *testing.T) []byte {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(key, "test only")
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(block)
}
