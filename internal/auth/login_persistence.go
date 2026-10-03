package auth

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/carlyleec/go-ssh-term/internal/database/queries"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type loginUser struct {
	account    queries.Account
	credential webauthn.Credential
}

func (u loginUser) WebAuthnID() []byte          { return u.account.WebauthnUserHandle }
func (u loginUser) WebAuthnName() string        { return u.account.DisplayName }
func (u loginUser) WebAuthnDisplayName() string { return u.account.DisplayName }
func (u loginUser) WebAuthnCredentials() []webauthn.Credential {
	return []webauthn.Credential{u.credential}
}

func restoreCredential(row queries.PasskeyCredential) (webauthn.Credential, error) {
	credential := webauthn.Credential{
		ID: row.CredentialID, PublicKey: row.PublicKey, AttestationType: row.AttestationType, AttestationFormat: row.AttestationFormat,
		Flags:         webauthn.NewCredentialFlags(protocol.AuthenticatorFlags(row.Flags)),
		Authenticator: webauthn.Authenticator{AAGUID: row.Aaguid, SignCount: uint32(row.SignCount), CloneWarning: row.CloneWarning, Attachment: protocol.AuthenticatorAttachment(row.Attachment)},
	}
	for _, transport := range row.Transports {
		credential.Transport = append(credential.Transport, protocol.AuthenticatorTransport(transport))
	}
	if err := json.Unmarshal(row.Attestation, &credential.Attestation); err != nil {
		return webauthn.Credential{}, err
	}
	if err := json.Unmarshal(row.Extensions, &credential.Extensions); err != nil {
		return webauthn.Credential{}, err
	}
	return credential, nil
}

func verifyLogin(ctx context.Context, pool *pgxpool.Pool, wa *webauthn.WebAuthn, session webauthn.SessionData, assertion *protocol.ParsedCredentialAssertionData) (queries.Account, error) {
	var account queries.Account
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := queries.New(tx)
		// Serialize verification and updates against the latest counter and flags.
		row, err := q.LockLoginCredential(ctx, queries.LockLoginCredentialParams{RpID: wa.Config.RPID, CredentialID: assertion.RawID, WebauthnUserHandle: assertion.Response.UserHandle})
		if errors.Is(err, pgx.ErrNoRows) {
			return errInvalidLogin
		}
		if err != nil {
			return err
		}
		credential, err := restoreCredential(row.PasskeyCredential)
		if err != nil {
			return err
		}
		user := loginUser{account: row.Account, credential: credential}
		_, verified, err := wa.ValidatePasskeyLogin(func([]byte, []byte) (webauthn.User, error) { return user, nil }, session, assertion)
		if err != nil {
			return errInvalidLogin
		}
		if err := q.UpdateLoginCredential(ctx, queries.UpdateLoginCredentialParams{
			AccountID: row.Account.ID, RpID: wa.Config.RPID, SignCount: int64(verified.Authenticator.SignCount),
			CloneWarning: verified.Authenticator.CloneWarning, Flags: int16(verified.Flags.ProtocolValue()),
		}); err != nil {
			return err
		}
		account = row.Account
		return nil
	})
	return account, err
}
