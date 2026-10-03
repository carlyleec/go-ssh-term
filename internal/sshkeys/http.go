package sshkeys

import (
	"database/sql"
	"encoding/json"
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

func NewHandler(db *sql.DB, encryption *Encryption, access *auth.Access) http.Handler {
	h := &handler{queries: queries.New(db), encryption: encryption}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/keys", h.upload)
	mux.HandleFunc("GET /api/keys", h.list)
	mux.HandleFunc("DELETE /api/keys/{id}", h.delete)
	return access.Require(mux)
}

func (h *handler) upload(w http.ResponseWriter, r *http.Request) {
	name, plain, status, err := readUpload(w, r)
	defer clear(plain)
	if err != nil {
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	fingerprint, err := Validate(plain)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	account, _ := auth.AccountFromContext(r.Context())
	id, err := uuid.NewRandom()
	if err != nil {
		keyError(w, "could not create SSH key; try again")
		return
	}
	encrypted, err := h.encryption.encrypt(id.String(), account.ID, plain)
	if err != nil {
		keyError(w, "could not encrypt SSH key; try again")
		return
	}
	clear(plain)
	metadata := keyMetadata{ID: id.String(), Name: name, PublicFingerprint: fingerprint, CreatedAt: time.Now().UTC()}
	ctx, cancel := sqlite.WorkContext(r.Context())
	defer cancel()
	if err := h.queries.CreateSSHKey(ctx, queries.CreateSSHKeyParams{
		ID: metadata.ID, AccountID: account.ID, Name: metadata.Name,
		PublicFingerprint: fingerprint, EncryptedPrivateKey: encrypted, CreatedAt: metadata.CreatedAt.UnixNano(),
	}); err != nil {
		keyError(w, "could not save SSH key; try again")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]keyMetadata{"key": metadata})
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	account, _ := auth.AccountFromContext(r.Context())
	ctx, cancel := sqlite.WorkContext(r.Context())
	defer cancel()
	rows, err := h.queries.ListSSHKeyMetadata(ctx, account.ID)
	if err != nil {
		keyError(w, "could not list SSH keys; try again")
		return
	}
	keys := make([]keyMetadata, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, keyMetadata{ID: row.ID, Name: row.Name, PublicFingerprint: row.PublicFingerprint, CreatedAt: time.Unix(0, row.CreatedAt).UTC()})
	}
	writeJSON(w, http.StatusOK, map[string][]keyMetadata{"keys": keys})
}

func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	account, _ := auth.AccountFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil || id.String() != r.PathValue("id") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "SSH key not found"})
		return
	}
	ctx, cancel := sqlite.WorkContext(r.Context())
	defer cancel()
	deleted, err := h.queries.DeleteSSHKey(ctx, queries.DeleteSSHKeyParams{ID: id.String(), AccountID: account.ID})
	if err != nil {
		keyError(w, "could not delete SSH key; try again")
		return
	}
	if deleted == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "SSH key not found"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

func keyError(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
