package connections

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/auth"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite/queries"
	"github.com/google/uuid"
	modernsqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type handler struct{ queries *queries.Queries }

func NewHandler(db *sql.DB) *handler { return &handler{queries: queries.New(db)} }

func metadata(row queries.SavedConnection) Connection {
	return Connection{ID: row.ID, Name: row.Name, Host: row.Host, Port: row.Port, Username: row.Username, SSHKeyID: row.SshKeyID,
		CreatedAt: time.Unix(0, row.CreatedAt).UTC(), UpdatedAt: time.Unix(0, row.UpdatedAt).UTC()}
}

func saveError(err error) error {
	var constraint *modernsqlite.Error
	if errors.As(err, &constraint) && constraint.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY {
		return failure(400, "select an owned SSH key")
	}
	return failure(503, "could not save connection; try again")
}

func (h *handler) create(ctx context.Context, input *CreateInput) (*ConnectionOutput, error) {
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
		Username: input.Body.Username, SshKeyID: input.Body.SSHKeyID, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		return nil, saveError(err)
	}
	return &ConnectionOutput{Body: ConnectionBody{Connection: metadata(row)}}, nil
}

func (h *handler) list(ctx context.Context, _ *struct{}) (*ListOutput, error) {
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

func (h *handler) update(ctx context.Context, input *UpdateInput) (*ConnectionOutput, error) {
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
		Username: input.Body.Username, SshKeyID: input.Body.SSHKeyID, UpdatedAt: time.Now().UTC().UnixNano()})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, failure(404, "connection not found")
	}
	if err != nil {
		return nil, saveError(err)
	}
	return &ConnectionOutput{Body: ConnectionBody{Connection: metadata(row)}}, nil
}

func (h *handler) delete(ctx context.Context, input *DeleteInput) (*struct{}, error) {
	if !canonicalID(input.ID) {
		return nil, failure(404, "connection not found")
	}
	account, _ := auth.AccountFromContext(ctx)
	ctx, cancel := sqlite.WorkContext(ctx)
	defer cancel()
	count, err := h.queries.DeleteSavedConnection(ctx, queries.DeleteSavedConnectionParams{ID: input.ID, AccountID: account.ID})
	if err != nil {
		return nil, failure(503, "could not delete connection; try again")
	}
	if count == 0 {
		return nil, failure(404, "connection not found")
	}
	return &struct{}{}, nil
}
