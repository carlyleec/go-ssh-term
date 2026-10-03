// Package sshkeys validates uploaded SSH credentials.
package sshkeys

import (
	"bytes"
	"crypto/ed25519"
	"encoding/pem"
	"errors"

	"golang.org/x/crypto/ssh"
)

const MaxUploadBytes = 16 * 1024

var (
	ErrTooLarge   = errors.New("SSH key file must be at most 16 KiB")
	ErrInvalid    = errors.New("invalid SSH private key")
	ErrFormat     = errors.New("only OpenSSH private-key format is supported")
	ErrAlgorithm  = errors.New("only Ed25519 private keys are supported")
	ErrPassphrase = errors.New("passphrase-protected SSH keys are not supported; upload a key without a passphrase")
	ErrContent    = errors.New("upload exactly one private key with no additional content")
)

// Validate checks the entire uploaded file and returns its SHA-256 public-key
// fingerprint. Errors contain no uploaded content or underlying parser details.
// Callers accepting streams must also bound reads before allocating the file.
func Validate(data []byte) (string, error) {
	if len(data) > MaxUploadBytes {
		return "", ErrTooLarge
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return "", ErrInvalid
	}
	// pem.Decode can skip text and malformed blocks before a valid block.
	// Require one envelope so such skipped content cannot be accepted.
	if !bytes.HasPrefix(data, []byte("-----BEGIN ")) {
		return "", ErrFormat
	}
	if bytes.Count(data, []byte("-----BEGIN ")) != 1 || bytes.Count(data, []byte("-----END ")) != 1 {
		return "", ErrContent
	}
	block, rest := pem.Decode(data)
	if block == nil {
		return "", ErrInvalid
	}
	if len(bytes.TrimSpace(rest)) != 0 {
		return "", ErrContent
	}
	if block.Type != "OPENSSH PRIVATE KEY" {
		return "", ErrFormat
	}
	if len(block.Headers) != 0 {
		return "", ErrInvalid
	}

	raw, err := ssh.ParseRawPrivateKey(data)
	if err != nil {
		var missing *ssh.PassphraseMissingError
		if errors.As(err, &missing) {
			return "", ErrPassphrase
		}
		return "", ErrInvalid
	}
	key, ok := raw.(*ed25519.PrivateKey)
	if !ok {
		return "", ErrAlgorithm
	}
	// The SSH parser checks length but not whether the cached public half
	// matches the seed. Reject keys that would produce unusable signatures.
	if !bytes.Equal(*key, ed25519.NewKeyFromSeed((*key).Seed())) {
		return "", ErrInvalid
	}
	public, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		return "", ErrInvalid
	}
	// The library permits trailing envelope bytes and does not compare the
	// outer public key with the private key. Enforce both for uploaded files.
	payload, ok := bytes.CutPrefix(block.Bytes, []byte("openssh-key-v1\x00"))
	var envelope struct {
		CipherName   string
		KDFName      string
		KDFOptions   string
		NumKeys      uint32
		PublicKey    []byte
		PrivateBlock []byte
	}
	if !ok || ssh.Unmarshal(payload, &envelope) != nil || !bytes.Equal(envelope.PublicKey, public.Marshal()) {
		return "", ErrInvalid
	}
	return ssh.FingerprintSHA256(public), nil
}
