package auth

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/config"
	"github.com/carlyleec/go-ssh-term/internal/database"
	"github.com/carlyleec/go-ssh-term/internal/database/testdb"
	"github.com/google/uuid"
)

func TestAccountSessionsSurviveDatabaseReopen(t *testing.T) {
	db, path := testdb.New(t)
	user, _ := loginIdentity(t)
	if err := saveRegistration(t.Context(), db, "localhost", registrationUser{ID: uuid.MustParse(user.account.ID), Handle: user.account.WebauthnUserHandle, DisplayName: user.account.DisplayName}, &user.credential); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{SessionLifetime: 12 * time.Hour}
	sessions, stop := NewSessions(cfg, db)
	first := accessCookie(t, sessions, user.account.ID, false)
	second := accessCookie(t, sessions, user.account.ID, false)
	ctx, err := sessions.Load(t.Context(), first.Value)
	if err != nil {
		t.Fatal(err)
	}
	deadline := sessions.Deadline(ctx)
	stop()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := database.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	sessions, stop = NewSessions(cfg, reopened)
	defer stop()
	access := NewAccess(sessions, reopened, "localhost", testOrigin)
	ctx, err = sessions.Load(context.Background(), first.Value)
	if err != nil || !sessions.Deadline(ctx).Equal(deadline) {
		t.Fatalf("deadline changed after reopening: %v", err)
	}
	if w := logoutRequest(access.Logout(), first, testOrigin); w.Code != 204 {
		t.Fatalf("logout after reopening: %d", w.Code)
	}
	for _, tc := range []struct {
		token  string
		status int
	}{{first.Value, 401}, {second.Value, 200}} {
		r := httptest.NewRequest("GET", "/api/auth/me", nil)
		cookie := *second
		cookie.Value = tc.token
		r.AddCookie(&cookie)
		w := httptest.NewRecorder()
		currentUserHandler(access).ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("session after reopen/logout: %d, want %d", w.Code, tc.status)
		}
	}
}
