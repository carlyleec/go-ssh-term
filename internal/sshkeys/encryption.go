package sshkeys

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
)

const encryptionVersion byte = 1

var errDecrypt = errors.New("stored SSH keys cannot be decrypted; restore the matching encryption-key volume and database backup")

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

// decrypt accepts version || nonce || ciphertext || tag. Authentication binds
// the payload to both its account and record, preventing ciphertext swapping.
func (e *Encryption) decrypt(id, accountID string, data []byte) ([]byte, error) {
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
