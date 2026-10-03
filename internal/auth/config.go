package auth

import (
	"net/http"

	"github.com/alexedwards/scs/pgxstore"
	"github.com/alexedwards/scs/v2"
	"github.com/carlyleec/go-ssh-term/internal/config"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewWebAuthn(cfg config.Config) (*webauthn.WebAuthn, error) {
	timeout := webauthn.TimeoutConfig{
		Enforce: true, Timeout: cfg.ChallengeLifetime, TimeoutUVD: cfg.ChallengeLifetime,
	}
	requireResidentKey := true
	return webauthn.New(&webauthn.Config{
		RPID:                  cfg.RPID,
		RPDisplayName:         "Browser SSH Gateway",
		RPOrigins:             []string{cfg.BrowserOrigin},
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			RequireResidentKey: &requireResidentKey,
			UserVerification:   protocol.VerificationRequired,
		},
		Timeouts: webauthn.TimeoutsConfig{Login: timeout, Registration: timeout},
	})
}

// NewSessions requires the sessions table and a shared pool. Call stopCleanup
// after HTTP shutdown and before closing the pool.
func NewSessions(cfg config.Config, pool *pgxpool.Pool) (*scs.SessionManager, func()) {
	store := pgxstore.New(pool)
	sessions := scs.New()
	sessions.Store = store
	sessions.Lifetime = cfg.SessionLifetime
	// Terminal activity does not refresh HTTP sessions; use an absolute deadline.
	sessions.IdleTimeout = 0
	sessions.Cookie.Name = "ssh_term_session"
	sessions.Cookie.Path = "/"
	sessions.Cookie.Domain = ""
	sessions.Cookie.HttpOnly = true
	sessions.Cookie.SameSite = http.SameSiteStrictMode
	sessions.Cookie.Secure = cfg.CookieSecure
	sessions.Cookie.Persist = true
	return sessions, store.StopCleanup
}
