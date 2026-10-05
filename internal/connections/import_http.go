package connections

import (
	"context"
	"github.com/google/uuid"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/auth"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite/queries"
)

type ImportSelection struct {
	Name             string `json:"name" maxLength:"64"`
	SSHKeyID         string `json:"ssh_key_id" maxLength:"36"`
	JumpConnectionID string `json:"jump_connection_id" maxLength:"36"`
	JumpUpdatedAt    string `json:"jump_updated_at" maxLength:"40"`
}
type ImportRequest struct {
	Config     string            `json:"config" maxLength:"65536"`
	Selections []ImportSelection `json:"selections,omitempty"`
}
type ImportIssue struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}
type ImportInput struct{ Body ImportRequest }
type ImportOutput struct{ Body ImportPreview }

func (h *Handler) PreviewImport(ctx context.Context, input *ImportInput) (*ImportOutput, error) {
	if len(input.Body.Config) > MaxImportBytes {
		return nil, failure(413, "config exceeds 64 KiB")
	}
	account, _ := auth.AccountFromContext(ctx)
	ctx, cancel := sqlite.WorkContext(ctx)
	defer cancel()
	preview, err := validateImport(ctx, h.queries, account.ID, input.Body)
	if err != nil {
		return nil, err
	}
	return &ImportOutput{Body: preview}, nil
}

// Both preview and confirmation use current owner-scoped metadata. Confirmation
// supplies transactional queries so validation and insertion share one snapshot.
func validateImport(ctx context.Context, q *queries.Queries, owner string, request ImportRequest) (ImportPreview, error) {
	result := parseImport(request.Config)
	if len(request.Selections) > MaxImportEntries {
		return result, failure(400, "select at most 100 entries")
	}
	if len(result.Diagnostics) != 0 {
		return result, nil
	}
	rows, err := q.ListSavedConnections(ctx, owner)
	if err != nil {
		return result, failure(503, "could not validate import; try again")
	}
	keys, err := q.ListSSHKeyMetadata(ctx, owner)
	if err != nil {
		return result, failure(503, "could not validate import; try again")
	}
	ownedKeys := map[string]bool{}
	for _, key := range keys {
		ownedKeys[key.ID] = true
	}
	existing := map[string]queries.SavedConnection{}
	names := map[string]bool{}
	for _, row := range rows {
		existing[row.ID] = row
		names[row.Name] = true
	}
	entries := map[string]ImportEntry{}
	for _, entry := range result.Entries {
		entries[entry.Name] = entry
	}
	selected := map[string]bool{}
	issue := func(name, message string) { result.Issues = append(result.Issues, ImportIssue{name, message}) }
	for _, selection := range request.Selections {
		if selected[selection.Name] {
			issue(selection.Name, "entry selected more than once")
		}
		selected[selection.Name] = true
	}
	for _, selection := range request.Selections {
		entry, ok := entries[selection.Name]
		if !ok {
			issue(selection.Name, "selected entry is not in this config")
			continue
		}
		if names[entry.Name] {
			issue(entry.Name, "a saved connection already uses this name")
		}
		if !ownedKeys[selection.SSHKeyID] {
			issue(entry.Name, "select an uploaded SSH key")
		}
		if entry.Jump == "" {
			if selection.JumpConnectionID != "" || selection.JumpUpdatedAt != "" {
				issue(entry.Name, "direct entries cannot have a jump mapping")
			}
			continue
		}
		if entry.Jump == entry.Name {
			issue(entry.Name, "a connection cannot jump through itself")
			continue
		}
		if selected[entry.Jump] {
			if entries[entry.Jump].Jump != "" {
				issue(entry.Name, "selected jump must be a direct connection")
			}
			if selection.JumpConnectionID != "" || selection.JumpUpdatedAt != "" {
				issue(entry.Name, "selected jump entry cannot also map to an existing connection")
			}
		} else {
			jump, exists := existing[selection.JumpConnectionID]
			if !exists || jump.Name != entry.Jump || jump.JumpConnectionID.Valid {
				issue(entry.Name, "select the jump entry or map an existing owned direct connection with that name")
				continue
			}
			if selection.JumpUpdatedAt != time.Unix(0, jump.UpdatedAt).UTC().Format(time.RFC3339Nano) {
				issue(entry.Name, "existing jump changed; refresh its mapping")
			}
		}
	}
	result.CanConfirm = len(request.Selections) > 0 && len(result.Issues) == 0
	return result, nil
}

func (h *Handler) ConfirmImport(ctx context.Context, input *ImportInput) (*ListOutput, error) {
	if len(input.Body.Config) > MaxImportBytes {
		return nil, failure(413, "config exceeds 64 KiB")
	}
	account, _ := auth.AccountFromContext(ctx)
	ctx, cancel := sqlite.WorkContext(ctx)
	defer cancel()
	// The configured immediate transaction serializes name checks with writers.
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, failure(503, "could not save import; try again")
	}
	defer tx.Rollback()
	q := h.queries.WithTx(tx)
	preview, err := validateImport(ctx, q, account.ID, input.Body)
	if err != nil {
		return nil, err
	}
	if !preview.CanConfirm {
		return nil, failure(409, "import is no longer valid; preview the selection again")
	}
	selections := map[string]ImportSelection{}
	ids := map[string]string{}
	for _, selection := range input.Body.Selections {
		selections[selection.Name] = selection
		id, err := uuid.NewRandom()
		if err != nil {
			return nil, failure(503, "could not save import; try again")
		}
		ids[selection.Name] = id.String()
	}
	saved := make([]Connection, 0, len(selections))
	now := time.Now().UTC().UnixNano()
	// Direct entries must exist before target foreign keys and graph triggers run.
	for _, withJump := range []bool{false, true} {
		for _, entry := range preview.Entries {
			selection, selected := selections[entry.Name]
			if !selected || (entry.Jump != "") != withJump {
				continue
			}
			var jump *string
			if withJump {
				id := ids[entry.Jump]
				if id == "" {
					id = selection.JumpConnectionID
				}
				jump = &id
			}
			row, err := q.CreateSavedConnection(ctx, queries.CreateSavedConnectionParams{
				ID: ids[entry.Name], AccountID: account.ID, Name: entry.Name, Host: entry.Host, Port: entry.Port,
				Username: entry.Username, SshKeyID: selection.SSHKeyID, JumpConnectionID: nullableJump(jump), CreatedAt: now, UpdatedAt: now,
			})
			if err != nil {
				return nil, saveError(err)
			}
			saved = append(saved, metadata(row))
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, failure(503, "could not save import; refresh connections before retrying")
	}
	return &ListOutput{Body: ConnectionsBody{Connections: saved}}, nil
}
