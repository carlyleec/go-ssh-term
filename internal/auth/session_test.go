package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-webauthn/webauthn/webauthn"
)

type failingSessionStore struct {
	scs.Store
	failCommit, failDelete, failFind bool
}

func (s *failingSessionStore) Commit(token string, data []byte, expiry time.Time) error {
	if s.failCommit {
		return errors.New("test session commit failure")
	}
	return s.Store.Commit(token, data, expiry)
}
func (s *failingSessionStore) Delete(token string) error {
	if s.failDelete {
		return errors.New("test session delete failure")
	}
	return s.Store.Delete(token)
}
func (s *failingSessionStore) Find(token string) ([]byte, bool, error) {
	if s.failFind {
		return nil, false, errors.New("test session load failure")
	}
	return s.Store.Find(token)
}

func (s *failingSessionStore) CommitCtx(ctx context.Context, token string, data []byte, expiry time.Time) error {
	if s.failCommit {
		return errors.New("test session commit failure")
	}
	if store, ok := s.Store.(scs.CtxStore); ok {
		return store.CommitCtx(ctx, token, data, expiry)
	}
	return s.Store.Commit(token, data, expiry)
}
func (s *failingSessionStore) DeleteCtx(ctx context.Context, token string) error {
	if s.failDelete {
		return errors.New("test session delete failure")
	}
	if store, ok := s.Store.(scs.CtxStore); ok {
		return store.DeleteCtx(ctx, token)
	}
	return s.Store.Delete(token)
}
func (s *failingSessionStore) FindCtx(ctx context.Context, token string) ([]byte, bool, error) {
	if s.failFind {
		return nil, false, errors.New("test session load failure")
	}
	if store, ok := s.Store.(scs.CtxStore); ok {
		return store.FindCtx(ctx, token)
	}
	return s.Store.Find(token)
}

func sessionAccount(t *testing.T, h *registration, cookie *http.Cookie) string {
	t.Helper()
	ctx, err := h.sessions.Load(context.Background(), cookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	return h.sessions.GetString(ctx, accountIDKey)
}

func TestRegistrationFreshSession(t *testing.T) {
	h, handler := registrationFixture(t)
	options, oldCookie := beginTest(t, handler, nil)
	ctx, err := h.sessions.Load(context.Background(), oldCookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	h.sessions.Put(ctx, "unrelated_anonymous_state", "discard me")
	h.sessions.SetDeadline(ctx, time.Now().Add(time.Minute))
	if _, _, err := h.sessions.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var savedUser registrationUser
	h.save = func(_ context.Context, u registrationUser, _ *webauthn.Credential) error { savedUser = u; return nil }
	before := time.Now()
	w := registrationRequestTest(handler, "finish", credentialResponse(t, options, testOrigin, "localhost", 0x45), testOrigin, oldCookie)
	if w.Code != 201 {
		t.Fatalf("finish: %d %s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value == oldCookie.Value || cookies[0].Value == "" {
		t.Fatal("session token was not replaced")
	}
	ctx, err = h.sessions.Load(context.Background(), cookies[0].Value)
	if err != nil {
		t.Fatal(err)
	}
	if h.sessions.GetString(ctx, accountIDKey) != savedUser.ID.String() || h.sessions.Exists(ctx, registrationBinding) || h.sessions.Exists(ctx, "unrelated_anonymous_state") {
		t.Fatal("authenticated session contains incorrect or anonymous state")
	}
	if h.sessions.Deadline(ctx).Before(before.Add(12 * time.Hour)) {
		t.Fatal("new session inherited the anonymous deadline")
	}
	if sessionAccount(t, h, oldCookie) != "" {
		t.Fatal("old cookie grants access")
	}
	if _, found, err := h.sessions.Store.Find(oldCookie.Value); err != nil || found {
		t.Fatal("old session still exists")
	}
	for _, path := range []string{"begin", "finish"} {
		if r := registrationRequestTest(handler, path, `{}`, testOrigin, cookies[0]); r.Code != 409 {
			t.Fatal("authenticated browser can overwrite its account")
		}
	}
}

func TestRegistrationPersistenceFailure(t *testing.T) {
	h, handler := registrationFixture(t)
	options, cookie := beginTest(t, handler, nil)
	h.save = func(context.Context, registrationUser, *webauthn.Credential) error {
		return errors.New("private database error")
	}
	body := credentialResponse(t, options, testOrigin, "localhost", 0x45)
	w := registrationRequestTest(handler, "finish", body, testOrigin, cookie)
	if w.Code != 503 || len(w.Result().Cookies()) != 0 || strings.Contains(w.Body.String(), "private") || sessionAccount(t, h, cookie) != "" {
		t.Fatalf("persistence failure was not safe: %d %s", w.Code, w.Body.String())
	}
	if retry := registrationRequestTest(handler, "finish", body, testOrigin, cookie); retry.Code != 400 {
		t.Fatal("persistence failure allowed challenge reuse")
	}
}

func TestRegistrationSessionFailure(t *testing.T) {
	for _, operation := range []string{"load", "delete", "commit"} {
		t.Run(operation, func(t *testing.T) {
			h, handler := registrationFixture(t)
			options, cookie := beginTest(t, handler, nil)
			saved := false
			h.save = func(context.Context, registrationUser, *webauthn.Credential) error { saved = true; return nil }
			store := &failingSessionStore{Store: h.sessions.Store, failFind: operation == "load", failDelete: operation == "delete", failCommit: operation == "commit"}
			h.sessions.Store = store
			body := credentialResponse(t, options, testOrigin, "localhost", 0x45)
			w := registrationRequestTest(handler, "finish", body, testOrigin, cookie)
			if w.Code != 503 || len(w.Result().Cookies()) != 0 || strings.Contains(w.Body.String(), `"account":`) {
				t.Fatalf("session failure sent success or a cookie: %d %s", w.Code, w.Body.String())
			}
			if saved != (operation != "load") {
				t.Fatal("unexpected persistence order")
			}
			h.sessions.Store = store.Store
			if sessionAccount(t, h, cookie) != "" {
				t.Fatal("old cookie grants access after failure")
			}
			if operation != "load" {
				if retry := registrationRequestTest(handler, "finish", body, testOrigin, cookie); retry.Code != 400 {
					t.Fatal("session failure allowed challenge reuse")
				}
			}
		})
	}
}
