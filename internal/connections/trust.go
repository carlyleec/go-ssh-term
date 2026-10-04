package connections

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/database/sqlite/queries"
	"golang.org/x/crypto/ssh"
)

type HostInspection struct {
	JumpConnectionID string `json:"jump_connection_id,omitempty" doc:"Present only when the fingerprint belongs to the selected bastion"`
	Hop              string `json:"hop" doc:"bastion or target"`

	Host               string `json:"host"`
	Port               int64  `json:"port"`
	State              string `json:"state" doc:"unknown, trusted, or changed"`
	Algorithm          string `json:"algorithm"`
	Fingerprint        string `json:"fingerprint"`
	TrustedFingerprint string `json:"trusted_fingerprint"`
}

// HostDecision binds a browser decision to exactly the endpoint and key shown.
type HostDecision struct {
	JumpConnectionID string `json:"jump_connection_id,omitempty" doc:"Echo the inspected bastion ID; omit for the target"`

	Host        string `json:"host" minLength:"1" maxLength:"253"`
	Port        int64  `json:"port" minimum:"1" maximum:"65535"`
	Fingerprint string `json:"fingerprint" minLength:"1" maxLength:"128"`
}

func fingerprint(data []byte) string {
	key, err := ssh.ParsePublicKey(data)
	if err != nil {
		return ""
	}
	return ssh.FingerprintSHA256(key)
}
func inspection(row queries.SavedConnection, key ssh.PublicKey, trusted []byte) HostInspection {
	state := "unknown"
	if trusted != nil {
		state = "changed"
		if bytes.Equal(trusted, key.Marshal()) {
			state = "trusted"
		}
	}
	return HostInspection{Hop: "target", Host: row.Host, Port: row.Port, State: state, Algorithm: key.Type(), Fingerprint: ssh.FingerprintSHA256(key), TrustedFingerprint: fingerprint(trusted)}
}
func (d *Dialer) Inspect(ctx context.Context, accountID, id string) (result HostInspection, err error) {
	defer func() { err = targetFailure(err) }()
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	row, err := d.owned(ctx, accountID, id)
	if err != nil {
		return HostInspection{}, err
	}

	jump, err := d.jumpConnection(ctx, row)
	if err != nil {
		return HostInspection{}, err
	}
	if jump != nil {
		trusted, err := d.trusted(ctx, *jump)
		if err != nil {
			return HostInspection{}, hopFailure("bastion", err)
		}
		key, err := d.probe(ctx, *jump, trusted)
		if err != nil {
			return HostInspection{}, hopFailure("bastion", err)
		}
		seen := inspection(*jump, key, trusted)
		if seen.State != "trusted" {
			seen.Hop, seen.JumpConnectionID = "bastion", jump.ID
			return seen, nil
		}
	}
	trusted, err := d.trusted(ctx, row)
	if err != nil {
		return HostInspection{}, err
	}
	key, err := d.probe(ctx, row, trusted)
	if err != nil {
		return HostInspection{}, err
	}
	return inspection(row, key, trusted), nil
}
func (d *Dialer) Approve(ctx context.Context, accountID, id string, decision HostDecision) (result HostInspection, err error) {
	defer func() {
		if decision.JumpConnectionID != "" {
			err = hopFailure("bastion", err)
		} else {
			err = targetFailure(err)
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	row, route, err := d.decisionHost(ctx, accountID, id, decision)
	if err != nil {
		return HostInspection{}, err
	}
	if row.Host != decision.Host || row.Port != decision.Port {
		return HostInspection{}, failure(409, "destination changed; inspect the host again")
	}
	trusted, err := d.trusted(ctx, row)
	if err != nil {
		return HostInspection{}, err
	}
	key, err := d.probe(ctx, row, trusted)
	if err != nil {
		return HostInspection{}, err
	}
	if ssh.FingerprintSHA256(key) != decision.Fingerprint {
		return HostInspection{}, failure(409, "SSH host fingerprint changed since inspection; inspect again")
	}
	// No transaction or socket survives a browser prompt. Serialize only the
	// final configuration check and trust insertion, never network operations.
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return HostInspection{}, failure(503, "could not save host trust; try again")
	}
	defer tx.Rollback()
	q := d.q.WithTx(tx)
	current, err := q.GetOwnedConnection(ctx, queries.GetOwnedConnectionParams{ID: row.ID, AccountID: accountID})
	if err != nil || current.Host != row.Host || current.Port != row.Port || !decisionRouteCurrent(ctx, q, route) {
		return HostInspection{}, failure(409, "destination changed; inspect the host again")
	}
	err = q.ApproveHostTrust(ctx, queries.ApproveHostTrustParams{AccountID: accountID, Host: row.Host, Port: row.Port, PublicKey: key.Marshal(), TrustedAt: time.Now().UTC().UnixNano()})
	if err != nil {
		return HostInspection{}, failure(503, "could not save host trust; try again")
	}
	saved, err := q.GetHostTrust(ctx, queries.GetHostTrustParams{AccountID: accountID, Host: row.Host, Port: row.Port})
	if err != nil {
		return HostInspection{}, failure(503, "could not read host trust; try again")
	}
	if !bytes.Equal(saved.PublicKey, key.Marshal()) {
		return HostInspection{}, failure(409, "SSH host key changed; explicitly reset trust before approving a replacement")
	}
	if err = tx.Commit(); err != nil {
		return HostInspection{}, failure(503, "could not save host trust; try again")
	}
	result = inspection(row, key, saved.PublicKey)
	if decision.JumpConnectionID != "" {
		result.Hop, result.JumpConnectionID = "bastion", row.ID
	}
	return result, nil
}
func (d *Dialer) ResetTrust(ctx context.Context, accountID, id string, decision HostDecision) (err error) {
	defer func() {
		if decision.JumpConnectionID != "" {
			err = hopFailure("bastion", err)
		} else {
			err = targetFailure(err)
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	row, route, err := d.decisionHost(ctx, accountID, id, decision)
	if err != nil {
		return err
	}
	if row.Host != decision.Host || row.Port != decision.Port {
		return failure(409, "destination changed; inspect the host again")
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return failure(503, "could not reset host trust; try again")
	}
	defer tx.Rollback()
	q := d.q.WithTx(tx)
	current, err := q.GetOwnedConnection(ctx, queries.GetOwnedConnectionParams{ID: row.ID, AccountID: accountID})
	if err != nil || current.Host != row.Host || current.Port != row.Port || !decisionRouteCurrent(ctx, q, route) {
		return failure(409, "destination changed; inspect the host again")
	}
	saved, err := q.GetHostTrust(ctx, queries.GetHostTrustParams{AccountID: accountID, Host: row.Host, Port: row.Port})
	if errors.Is(err, sql.ErrNoRows) {
		return failure(409, "host trust already removed; inspect the host again")
	}
	if err != nil {
		return failure(503, "could not load host trust; try again")
	}
	if fingerprint(saved.PublicKey) != decision.Fingerprint {
		return failure(409, "stored fingerprint changed; inspect the host again")
	}
	_, err = q.ResetHostTrust(ctx, queries.ResetHostTrustParams{AccountID: accountID, Host: row.Host, Port: row.Port, PublicKey: saved.PublicKey})
	if err != nil {
		return failure(503, "could not reset host trust; try again")
	}
	if err = tx.Commit(); err != nil {
		return failure(503, "could not reset host trust; try again")
	}
	return nil
}

// Bastion decisions stay bound to the target's current owned jump reference.
func (d *Dialer) decisionHost(ctx context.Context, accountID, id string, decision HostDecision) (queries.SavedConnection, queries.SavedConnection, error) {
	route, err := d.owned(ctx, accountID, id)
	if err != nil {
		return route, route, err
	}
	if decision.JumpConnectionID == "" {
		return route, route, nil
	}
	if !route.JumpConnectionID.Valid || route.JumpConnectionID.String != decision.JumpConnectionID {
		return route, route, failure(409, "jump connection changed; inspect again")
	}
	jump, err := d.jumpConnection(ctx, route)
	if err != nil {
		return route, route, err
	}
	return *jump, route, nil
}
func decisionRouteCurrent(ctx context.Context, q *queries.Queries, route queries.SavedConnection) bool {
	current, err := q.GetOwnedConnection(ctx, queries.GetOwnedConnectionParams{ID: route.ID, AccountID: route.AccountID})
	return err == nil && current.Host == route.Host && current.Port == route.Port && current.JumpConnectionID == route.JumpConnectionID
}
