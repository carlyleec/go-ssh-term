package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/database/sqlite"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite/queries"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
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
		Authenticator: webauthn.Authenticator{AAGUID: row.Aaguid, SignCount: uint32(row.SignCount), CloneWarning: row.CloneWarning != 0, Attachment: protocol.AuthenticatorAttachment(row.Attachment)},
	}
	var transports []string
	if err := json.Unmarshal([]byte(row.Transports), &transports); err != nil {
		return webauthn.Credential{}, err
	}
	for _, transport := range transports {
		credential.Transport = append(credential.Transport, protocol.AuthenticatorTransport(transport))
	}
	if err := json.Unmarshal([]byte(row.Attestation), &credential.Attestation); err != nil {
		return webauthn.Credential{}, err
	}
	if err := json.Unmarshal([]byte(row.Extensions), &credential.Extensions); err != nil {
		return webauthn.Credential{}, err
	}
	return credential, nil
}

func verifyLogin(ctx context.Context, pool *sql.DB, wa *webauthn.WebAuthn, session webauthn.SessionData, assertion *protocol.ParsedCredentialAssertionData) (queries.Account, error) {
	ctx, cancel := sqlite.WorkContext(ctx)
	defer cancel()
	var account queries.Account
	tx, err := pool.BeginTx(ctx, nil)
	if err != nil {
		return account, err
	}
	defer tx.Rollback()
	// BEGIN IMMEDIATE reserves the writer before reading the current credential.
	q := queries.New(tx)
	row, err := q.GetLoginCredential(ctx, queries.GetLoginCredentialParams{RpID: wa.Config.RPID, CredentialID: assertion.RawID, WebauthnUserHandle: assertion.Response.UserHandle})
	if errors.Is(err, sql.ErrNoRows) {
		return account, errInvalidLogin
	}
	if err != nil {
		return account, err
	}
	credential, err := restoreCredential(row.PasskeyCredential)
	if err != nil {
		return account, err
	}
	user := loginUser{account: row.Account, credential: credential}
	_, verified, err := wa.ValidatePasskeyLogin(func([]byte, []byte) (webauthn.User, error) { return user, nil }, session, assertion)
	if err != nil {
		return account, errInvalidLogin
	}
	if err := q.UpdateLoginCredential(ctx, queries.UpdateLoginCredentialParams{
		AccountID: row.Account.ID, RpID: wa.Config.RPID, SignCount: int64(verified.Authenticator.SignCount),
		CloneWarning: boolInteger(verified.Authenticator.CloneWarning), Flags: int64(verified.Flags.ProtocolValue()),
		LastUsedAt: sql.NullInt64{Int64: time.Now().UTC().UnixNano(), Valid: true},
	}); err != nil {
		return account, err
	}
	if err := tx.Commit(); err != nil {
		return account, err
	}
	return row.Account, nil
}
