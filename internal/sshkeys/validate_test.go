package sshkeys_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"testing"

	"github.com/carlyleec/go-ssh-term/internal/sshkeys"
	"golang.org/x/crypto/ssh"
)

func TestValidate(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	valid := marshalKey(t, private)
	sshPublic, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(sshPublic.Marshal())
	wantFingerprint := "SHA256:" + base64.RawStdEncoding.EncodeToString(digest[:])
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"ordinary", valid},
		{"surrounding whitespace", append([]byte(" \t\r\n"), append(bytes.Clone(valid), []byte(" \t\r\n")...)...)},
		{"CRLF", bytes.ReplaceAll(valid, []byte("\n"), []byte("\r\n"))},
		{"no final newline", bytes.TrimSpace(valid)},
		{"at size limit", append(bytes.Clone(valid), bytes.Repeat([]byte(" "), sshkeys.MaxUploadBytes-len(valid))...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := bytes.Clone(tc.data)
			fingerprint, err := sshkeys.Validate(tc.data)
			if err != nil || fingerprint != wantFingerprint {
				t.Fatalf("Validate = %q, %v; want %q", fingerprint, err, wantFingerprint)
			}
			if !bytes.Equal(before, tc.data) {
				t.Fatal("Validate modified the upload")
			}
		})
	}
}

func TestValidateRejects(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	valid := marshalKey(t, private)
	protected, err := ssh.MarshalPrivateKeyWithPassphrase(private, "test only", []byte("test passphrase"))
	if err != nil {
		t.Fatal(err)
	}
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	badPrivate := bytes.Clone(private)
	badPrivate[ed25519.SeedSize] ^= 1
	headers, _ := pem.Decode(valid)
	headers.Headers = map[string]string{"Comment": "unexpected header"}
	mismatchedPublic := bytes.Replace(headers.Bytes, []byte(private.Public().(ed25519.PublicKey)), bytes.Repeat([]byte{1}, ed25519.PublicKeySize), 1)
	for _, tc := range []struct {
		name string
		data []byte
		want error
	}{
		{"empty", nil, sshkeys.ErrInvalid},
		{"whitespace", []byte(" \t\r\n"), sshkeys.ErrInvalid},
		{"garbage", []byte("not a private key"), sshkeys.ErrFormat},
		{"public only", ssh.MarshalAuthorizedKey(mustPublic(t, private)), sshkeys.ErrFormat},
		{"PuTTY", []byte("PuTTY-User-Key-File-3: ssh-ed25519\n"), sshkeys.ErrFormat},
		{"PKCS8 Ed25519", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}), sshkeys.ErrFormat},
		{"PKCS1 RSA", pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rsaKey)}), sshkeys.ErrFormat},
		{"OpenSSH RSA", marshalKey(t, rsaKey), sshkeys.ErrAlgorithm},
		{"OpenSSH ECDSA", marshalKey(t, ecKey), sshkeys.ErrAlgorithm},
		{"protected", pem.EncodeToMemory(protected), sshkeys.ErrPassphrase},
		{"oversized", append(bytes.Clone(valid), bytes.Repeat([]byte(" "), sshkeys.MaxUploadBytes+1-len(valid))...), sshkeys.ErrTooLarge},
		{"oversized garbage", bytes.Repeat([]byte("x"), sshkeys.MaxUploadBytes+1), sshkeys.ErrTooLarge},
		{"two keys", append(bytes.Clone(valid), valid...), sshkeys.ErrContent},
		{"leading text", append([]byte("unexpected text\n"), valid...), sshkeys.ErrFormat},
		{"trailing text", append(bytes.Clone(valid), []byte("unexpected text")...), sshkeys.ErrContent},
		{"malformed block before key", append([]byte("-----BEGIN OPENSSH PRIVATE KEY-----\ninvalid\n-----END OPENSSH PRIVATE KEY-----\n"), valid...), sshkeys.ErrContent},
		{"PEM headers", pem.EncodeToMemory(headers), sshkeys.ErrInvalid},
		{"bad base64", []byte("-----BEGIN OPENSSH PRIVATE KEY-----\n!\n-----END OPENSSH PRIVATE KEY-----"), sshkeys.ErrInvalid},
		{"bad SSH payload", pem.EncodeToMemory(&pem.Block{Type: "OPENSSH PRIVATE KEY", Bytes: []byte("invalid")}), sshkeys.ErrInvalid},
		{"trailing binary content", pem.EncodeToMemory(&pem.Block{Type: "OPENSSH PRIVATE KEY", Bytes: append(bytes.Clone(headers.Bytes), 0)}), sshkeys.ErrInvalid},
		{"mismatched outer public key", pem.EncodeToMemory(&pem.Block{Type: "OPENSSH PRIVATE KEY", Bytes: mismatchedPublic}), sshkeys.ErrInvalid},
		{"truncated payload", pem.EncodeToMemory(&pem.Block{Type: "OPENSSH PRIVATE KEY", Bytes: headers.Bytes[:len(headers.Bytes)/2]}), sshkeys.ErrInvalid},
		{"mismatched private key", marshalKey(t, ed25519.PrivateKey(badPrivate)), sshkeys.ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fingerprint, err := sshkeys.Validate(tc.data)
			if !errors.Is(err, tc.want) || fingerprint != "" {
				t.Fatalf("Validate = %q, %v; want empty fingerprint and %v", fingerprint, err, tc.want)
			}
		})
	}
}

func marshalKey(t *testing.T, key any) []byte {
	t.Helper()
	block, err := ssh.MarshalPrivateKey(key, "test only")
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(block)
}

func mustPublic(t *testing.T, key ed25519.PrivateKey) ssh.PublicKey {
	t.Helper()
	public, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		t.Fatal(err)
	}
	return public
}
