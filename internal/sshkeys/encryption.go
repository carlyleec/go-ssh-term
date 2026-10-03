package sshkeys

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"

	"github.com/google/uuid"
)

const encryptionVersion byte = 1

var errDecrypt = errors.New("stored SSH keys cannot be decrypted; restore the matching encryption-key volume and database backup")
var errEncrypt = errors.New("cannot encrypt SSH key; check encryption initialization and record identity")

// Encryption holds the application key in the standard library's cipher state.
// Never serialize this value or expose it through an API.
type Encryption struct {
	aead cipher.AEAD
}

func newEncryption(key []byte) (*Encryption, error) {
	if len(key) != 32 {
		return nil, errors.New("application encryption key must contain exactly 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("cannot initialize application encryption")
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return nil, errors.New("cannot initialize application encryption")
	}
	return &Encryption{aead: aead}, nil
}

// encrypt preserves the supplied bytes. Callers validate the uploaded key first
// and clear their plaintext buffer after use. The returned payload is for storage,
// never an API response. Seal generates and prefixes a fresh random nonce.
func (e *Encryption) encrypt(id, accountID string, plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return nil, ErrInvalid
	}
	if len(plain) > MaxUploadBytes {
		return nil, ErrTooLarge
	}
	if e == nil || e.aead == nil || !validEncryptionIdentity(id, accountID) {
		return nil, errEncrypt
	}
	return e.aead.Seal([]byte{encryptionVersion}, nil, plain, encryptionContext(id, accountID)), nil
}

// decrypt accepts version || nonce || ciphertext || tag. Authentication binds
// the payload to both its account and record, preventing ciphertext swapping.
func (e *Encryption) decrypt(id, accountID string, data []byte) ([]byte, error) {
	if e == nil || e.aead == nil || !validEncryptionIdentity(id, accountID) {
		return nil, errDecrypt
	}
	if len(data) < 1+e.aead.Overhead() || len(data) > 1+e.aead.Overhead()+MaxUploadBytes || data[0] != encryptionVersion {
		return nil, errDecrypt
	}
	plain, err := e.aead.Open(nil, nil, data[1:], encryptionContext(id, accountID))
	if err != nil || len(plain) == 0 {
		clear(plain)
		return nil, errDecrypt
	}
	return plain, nil
}

func encryptionContext(id, accountID string) []byte {
	return []byte("go-ssh-term:ssh-key:v1\x00" + accountID + "\x00" + id)
}

// Match the database's canonical UUID representation so context fields cannot
// contain separators or alternate spellings of the same identity.
func validEncryptionIdentity(id, accountID string) bool {
	for _, value := range []string{id, accountID} {
		parsed, err := uuid.Parse(value)
		if err != nil || parsed.String() != value {
			return false
		}
	}
	return true
}
