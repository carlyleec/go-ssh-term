package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/database/sqlite"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite/queries"
	"github.com/go-webauthn/webauthn/webauthn"
)

func saveRegistration(ctx context.Context, pool *sql.DB, rpID string, user registrationUser, credential *webauthn.Credential) error {
	ctx, cancel := sqlite.WorkContext(ctx)
	defer cancel()
	attestation, err := json.Marshal(credential.Attestation)
	if err != nil {
		return err
	}
	extensions, err := json.Marshal(credential.Extensions)
	if err != nil {
		return err
	}
	transports := make([]string, len(credential.Transport))
	for i, transport := range credential.Transport {
		transports[i] = string(transport)
	}
	encodedTransports, err := json.Marshal(transports)
	if err != nil {
		return err
	}
	tx, err := pool.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := queries.New(tx)
	now := time.Now().UTC().UnixNano()
	if err := q.CreateAccount(ctx, queries.CreateAccountParams{
		ID: user.ID.String(), DisplayName: user.DisplayName, RpID: rpID, WebauthnUserHandle: user.Handle, CreatedAt: now,
	}); err != nil {
		return err
	}
	if err := q.CreatePasskeyCredential(ctx, queries.CreatePasskeyCredentialParams{
		AccountID: user.ID.String(), RpID: rpID, CredentialID: credential.ID, PublicKey: credential.PublicKey,
		AttestationType: credential.AttestationType, AttestationFormat: credential.AttestationFormat,
		Transports: string(encodedTransports), Flags: int64(credential.Flags.ProtocolValue()),
		Aaguid: credential.Authenticator.AAGUID, SignCount: int64(credential.Authenticator.SignCount),
		CloneWarning: boolInteger(credential.Authenticator.CloneWarning), Attachment: string(credential.Authenticator.Attachment),
		Attestation: string(attestation), Extensions: string(extensions), CreatedAt: now,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func boolInteger(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
