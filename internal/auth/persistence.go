package auth

import (
	"context"
	"encoding/json"

	"github.com/carlyleec/go-ssh-term/internal/database/queries"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func saveRegistration(ctx context.Context, pool *pgxpool.Pool, rpID string, user registrationUser, credential *webauthn.Credential) error {
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
	id := pgtype.UUID{Bytes: user.ID, Valid: true}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := queries.New(tx)
		if err := q.CreateAccount(ctx, queries.CreateAccountParams{
			ID: id, DisplayName: user.DisplayName, RpID: rpID, WebauthnUserHandle: user.Handle,
		}); err != nil {
			return err
		}
		return q.CreatePasskeyCredential(ctx, queries.CreatePasskeyCredentialParams{
			AccountID: id, RpID: rpID, CredentialID: credential.ID, PublicKey: credential.PublicKey,
			AttestationType: credential.AttestationType, AttestationFormat: credential.AttestationFormat,
			Transports: transports, Flags: int16(credential.Flags.ProtocolValue()),
			Aaguid: credential.Authenticator.AAGUID, SignCount: int64(credential.Authenticator.SignCount),
			CloneWarning: credential.Authenticator.CloneWarning, Attachment: string(credential.Authenticator.Attachment),
			Attestation: attestation, Extensions: extensions,
		})
	})
}
