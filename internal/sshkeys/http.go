package sshkeys

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/carlyleec/go-ssh-term/internal/auth"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite"
	"github.com/carlyleec/go-ssh-term/internal/database/sqlite/queries"
	"github.com/google/uuid"
)

const maxUploadRequestBytes = 32 * 1024
const maxNameCharacters = 64

type keyMetadata struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	PublicFingerprint string    `json:"public_fingerprint"`
	CreatedAt         time.Time `json:"created_at"`
}

type handler struct {
	queries    *queries.Queries
	encryption *Encryption
}

func NewHandler(db *sql.DB, encryption *Encryption) *handler {
	return &handler{queries: queries.New(db), encryption: encryption}
}

func (h *handler) upload(ctx context.Context, input *UploadInput) (*UploadOutput, error) {
	r, w := input.request, input.writer
	name, plain, status, err := readUpload(w, r)
	defer clear(plain)
	if err != nil {
		return nil, &KeyErrorBody{Message: err.Error(), status: status}
	}
	fingerprint, err := Validate(plain)
	if err != nil {
		return nil, &KeyErrorBody{Message: err.Error(), status: http.StatusBadRequest}
	}
	account, _ := auth.AccountFromContext(ctx)
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, &KeyErrorBody{Message: "could not create SSH key; try again", status: http.StatusServiceUnavailable}
	}
	encrypted, err := h.encryption.encrypt(id.String(), account.ID, plain)
	if err != nil {
		return nil, &KeyErrorBody{Message: "could not encrypt SSH key; try again", status: http.StatusServiceUnavailable}
	}
	clear(plain)
	metadata := keyMetadata{ID: id.String(), Name: name, PublicFingerprint: fingerprint, CreatedAt: time.Now().UTC()}
	ctx, cancel := sqlite.WorkContext(ctx)
	defer cancel()
	if err := h.queries.CreateSSHKey(ctx, queries.CreateSSHKeyParams{
		ID: metadata.ID, AccountID: account.ID, Name: metadata.Name,
		PublicFingerprint: fingerprint, EncryptedPrivateKey: encrypted, CreatedAt: metadata.CreatedAt.UnixNano(),
	}); err != nil {
		return nil, &KeyErrorBody{Message: "could not save SSH key; try again", status: http.StatusServiceUnavailable}
	}
	return &UploadOutput{Body: KeyBody{Key: metadata}}, nil
}

func (h *handler) list(ctx context.Context, _ *struct{}) (*ListOutput, error) {
	account, _ := auth.AccountFromContext(ctx)
	ctx, cancel := sqlite.WorkContext(ctx)
	defer cancel()
	rows, err := h.queries.ListSSHKeyMetadata(ctx, account.ID)
	if err != nil {
		return nil, &KeyErrorBody{Message: "could not list SSH keys; try again", status: http.StatusServiceUnavailable}
	}
	keys := make([]keyMetadata, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, keyMetadata{ID: row.ID, Name: row.Name, PublicFingerprint: row.PublicFingerprint, CreatedAt: time.Unix(0, row.CreatedAt).UTC()})
	}
	return &ListOutput{Body: KeysBody{Keys: keys}}, nil
}

func (h *handler) delete(ctx context.Context, input *DeleteInput) (*struct{}, error) {
	account, _ := auth.AccountFromContext(ctx)
	id, err := uuid.Parse(input.ID)
	if err != nil || id.String() != input.ID {
		return nil, &KeyErrorBody{Message: "SSH key not found", status: http.StatusNotFound}
	}
	ctx, cancel := sqlite.WorkContext(ctx)
	defer cancel()
	deleted, err := h.queries.DeleteSSHKey(ctx, queries.DeleteSSHKeyParams{ID: id.String(), AccountID: account.ID})
	if err != nil {
		return nil, &KeyErrorBody{Message: "could not delete SSH key; try again", status: http.StatusServiceUnavailable}
	}
	if deleted == 0 {
		return nil, &KeyErrorBody{Message: "SSH key not found", status: http.StatusNotFound}
	}
	return &struct{}{}, nil
}

// Stream parts rather than using ParseMultipartForm, which may spool private
// material to temporary files. Bound each read as well as the whole request.
func readUpload(w http.ResponseWriter, r *http.Request) (name string, plain []byte, status int, err error) {
	fail := func(code int, message string) (string, []byte, int, error) {
		clear(plain)
		return "", nil, code, errors.New(message)
	}
	readFailure := func(err error) (string, []byte, int, error) {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return fail(http.StatusRequestEntityTooLarge, "upload request must be at most 32 KiB")
		}
		return fail(http.StatusBadRequest, "invalid multipart upload")
	}
	if r.ContentLength > maxUploadRequestBytes {
		return fail(http.StatusRequestEntityTooLarge, "upload request must be at most 32 KiB")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadRequestBytes)
	mediaType, _, parseErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if parseErr != nil || mediaType != "multipart/form-data" {
		return fail(http.StatusUnsupportedMediaType, "Content-Type must be multipart/form-data with a boundary")
	}
	reader, err := r.MultipartReader()
	if err != nil {
		return fail(http.StatusUnsupportedMediaType, "Content-Type must be multipart/form-data with a boundary")
	}
	seenName, seenKey := false, false
	for {
		part, err := reader.NextRawPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return readFailure(err)
		}
		if part.Header.Get("Content-Transfer-Encoding") != "" {
			return fail(http.StatusBadRequest, "encoded multipart parts are not supported")
		}
		switch part.FormName() {
		case "name":
			if seenName || part.FileName() != "" {
				return fail(http.StatusBadRequest, "provide one name field")
			}
			seenName = true
			value, err := io.ReadAll(io.LimitReader(part, maxNameCharacters*utf8.UTFMax+1))
			if err != nil {
				return readFailure(err)
			}
			if len(value) > maxNameCharacters*utf8.UTFMax {
				return fail(http.StatusBadRequest, "name must contain 1 to 64 characters without control characters")
			}
			name = strings.TrimSpace(string(value))
			if !utf8.ValidString(name) || name == "" || utf8.RuneCountInString(name) > maxNameCharacters || strings.ContainsFunc(name, unicode.IsControl) {
				return fail(http.StatusBadRequest, "name must contain 1 to 64 characters without control characters")
			}
		case "private_key":
			if seenKey {
				return fail(http.StatusBadRequest, "provide one private_key file")
			}
			seenKey = true
			plain, err = io.ReadAll(io.LimitReader(part, MaxUploadBytes+1))
			if err != nil {
				return readFailure(err)
			}
			if len(plain) > MaxUploadBytes {
				return fail(http.StatusRequestEntityTooLarge, ErrTooLarge.Error())
			}
		default:
			return fail(http.StatusBadRequest, "only name and private_key fields are accepted")
		}
		if err := part.Close(); err != nil {
			return readFailure(err)
		}
	}
	// Multipart parsing stops at the closing boundary. Consume any epilogue
	// through MaxBytesReader so chunked bodies cannot bypass the request limit.
	if _, err := io.Copy(io.Discard, r.Body); err != nil {
		return readFailure(err)
	}
	if !seenName || !seenKey {
		return fail(http.StatusBadRequest, "name and private_key are required")
	}
	return name, plain, 0, nil
}
