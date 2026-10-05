package connections

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/auth"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite/queries"
	"github.com/google/uuid"
	modernsqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type Handler struct {
	queries *queries.Queries
	db      *sql.DB
}

func NewHandler(db *sql.DB) *Handler { return &Handler{queries: queries.New(db), db: db} }

func metadata(row queries.SavedConnection) Connection {
	var jump *string
	if row.JumpConnectionID.Valid {
		jump = &row.JumpConnectionID.String
	}
	return Connection{JumpConnectionID: jump, ID: row.ID, Name: row.Name, Host: row.Host, Port: row.Port, Username: row.Username, SSHKeyID: row.SshKeyID,
		CreatedAt: time.Unix(0, row.CreatedAt).UTC(), UpdatedAt: time.Unix(0, row.UpdatedAt).UTC()}
}

func nullableJump(id *string) sql.NullString {
	if id == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *id, Valid: true}
}

func saveError(err error) error {
	var constraint *modernsqlite.Error
	if errors.As(err, &constraint) && constraint.Code() == sqlite3.SQLITE_CONSTRAINT_TRIGGER && strings.Contains(constraint.Error(), "invalid_jump_connection") {
		return failure(400, "select an owned direct jump connection; self references and multiple hops are not allowed")
	}
	if errors.As(err, &constraint) && constraint.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY {
		return failure(400, "select an owned SSH key")
	}
	return failure(503, "could not save connection; try again")
}

func (h *Handler) Create(ctx context.Context, input *CreateInput) (*ConnectionOutput, error) {
	if err := input.Body.normalize(); err != nil {
		return nil, err
	}
	account, _ := auth.AccountFromContext(ctx)
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, failure(503, "could not create connection; try again")
	}
	now := time.Now().UTC().UnixNano()
	ctx, cancel := sqlite.WorkContext(ctx)
	defer cancel()
	row, err := h.queries.CreateSavedConnection(ctx, queries.CreateSavedConnectionParams{
		ID: id.String(), AccountID: account.ID, Name: input.Body.Name, Host: input.Body.Host, Port: input.Body.Port,
		Username: input.Body.Username, SshKeyID: input.Body.SSHKeyID, JumpConnectionID: nullableJump(input.Body.JumpConnectionID), CreatedAt: now, UpdatedAt: now})
	if err != nil {
		return nil, saveError(err)
	}
	return &ConnectionOutput{Body: ConnectionBody{Connection: metadata(row)}}, nil
}

func (h *Handler) List(ctx context.Context, _ *struct{}) (*ListOutput, error) {
	account, _ := auth.AccountFromContext(ctx)
	ctx, cancel := sqlite.WorkContext(ctx)
	defer cancel()
	rows, err := h.queries.ListSavedConnections(ctx, account.ID)
	if err != nil {
		return nil, failure(503, "could not list connections; try again")
	}
	result := make([]Connection, 0, len(rows))
	for _, row := range rows {
		result = append(result, metadata(row))
	}
	return &ListOutput{Body: ConnectionsBody{Connections: result}}, nil
}

func (h *Handler) Update(ctx context.Context, input *UpdateInput) (*ConnectionOutput, error) {
	if !canonicalID(input.ID) {
		return nil, failure(404, "connection not found")
	}
	if err := input.Body.normalize(); err != nil {
		return nil, err
	}
	account, _ := auth.AccountFromContext(ctx)
	ctx, cancel := sqlite.WorkContext(ctx)
	defer cancel()
	row, err := h.queries.UpdateSavedConnection(ctx, queries.UpdateSavedConnectionParams{
		ID: input.ID, AccountID: account.ID, Name: input.Body.Name, Host: input.Body.Host, Port: input.Body.Port,
		Username: input.Body.Username, SshKeyID: input.Body.SSHKeyID, JumpConnectionID: nullableJump(input.Body.JumpConnectionID), UpdatedAt: time.Now().UTC().UnixNano()})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, failure(404, "connection not found")
	}
	if err != nil {
		return nil, saveError(err)
	}
	return &ConnectionOutput{Body: ConnectionBody{Connection: metadata(row)}}, nil
}

func (h *Handler) Delete(ctx context.Context, input *DeleteInput) (*struct{}, error) {
	if !canonicalID(input.ID) {
		return nil, failure(404, "connection not found")
	}
	account, _ := auth.AccountFromContext(ctx)
	ctx, cancel := sqlite.WorkContext(ctx)
	defer cancel()
	count, err := h.queries.DeleteSavedConnection(ctx, queries.DeleteSavedConnectionParams{ID: input.ID, AccountID: account.ID})
	if err != nil {
		var constraint *modernsqlite.Error
		if errors.As(err, &constraint) && constraint.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY {
			return nil, failure(409, "connection is used as a jump; update or delete dependent connections first")
		}
		return nil, failure(503, "could not delete connection; try again")
	}
	if count == 0 {
		return nil, failure(404, "connection not found")
	}
	return &struct{}{}, nil
}
