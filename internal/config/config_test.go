package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	for _, key := range []string{"HTTP_ADDR", "DATABASE_PATH", "BROWSER_ORIGIN", "SHUTDOWN_TIMEOUT", "SESSION_LIFETIME", "CHALLENGE_LIFETIME"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("DATABASE_PATH", "/data/gateway.db")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.BrowserOrigin != "http://localhost:8080" || cfg.ShutdownTimeout != 5*time.Second || cfg.RPID != "localhost" || cfg.CookieSecure || cfg.SessionLifetime != 12*time.Hour || cfg.ChallengeLifetime != 5*time.Minute {
		t.Fatal("unexpected defaults")
	}
}

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("DATABASE_PATH", "/data/gateway.db")
	t.Setenv("BROWSER_ORIGIN", "http://localhost:5173")
	t.Setenv("SHUTDOWN_TIMEOUT", "5s")
	t.Setenv("SESSION_LIFETIME", "12h")
	t.Setenv("CHALLENGE_LIFETIME", "5m")
}

func TestLoad(t *testing.T) {
	setValidEnv(t)
	t.Setenv("HTTP_ADDR", "[::1]:9000")
	t.Setenv("DATABASE_PATH", "/data/gateway.db")
	t.Setenv("SHUTDOWN_TIMEOUT", "250ms")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != "[::1]:9000" || cfg.BrowserOrigin != "http://localhost:5173" || cfg.ShutdownTimeout != 250*time.Millisecond || cfg.DatabasePath == "" {
		t.Fatalf("unexpected configuration fields")
	}
}

func TestInvalidConfig(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"HTTP_ADDR", "8080"}, {"HTTP_ADDR", ":0"}, {"HTTP_ADDR", ":65536"}, {"HTTP_ADDR", ""},
		{"BROWSER_ORIGIN", "http://localhost:5173/"}, {"BROWSER_ORIGIN", "http://user:secret@localhost"},
		{"BROWSER_ORIGIN", "http://localhost?secret=yes"}, {"BROWSER_ORIGIN", "http://localhost#fragment"},
		{"BROWSER_ORIGIN", "http://127.0.0.1:8080"}, {"BROWSER_ORIGIN", "https://127.0.0.1"}, {"BROWSER_ORIGIN", "http://[::1]:8080"}, {"BROWSER_ORIGIN", "http://example.com"},
		{"SESSION_LIFETIME", "0s"}, {"SESSION_LIFETIME", "-1h"}, {"SESSION_LIFETIME", "500ms"}, {"SESSION_LIFETIME", "bad"}, {"SESSION_LIFETIME", ""},
		{"CHALLENGE_LIFETIME", "0s"}, {"CHALLENGE_LIFETIME", "-1m"}, {"CHALLENGE_LIFETIME", "1ns"}, {"CHALLENGE_LIFETIME", "bad"}, {"CHALLENGE_LIFETIME", ""},
		{"BROWSER_ORIGIN", "ftp://localhost"}, {"BROWSER_ORIGIN", "http://localhost:99999"}, {"BROWSER_ORIGIN", ""},
		{"DATABASE_PATH", "postgres://user:secret%zz@db/app"}, {"DATABASE_PATH", "https://db/app"}, {"DATABASE_PATH", ""},
		{"SHUTDOWN_TIMEOUT", "0s"}, {"SHUTDOWN_TIMEOUT", "-1s"}, {"SHUTDOWN_TIMEOUT", "five"}, {"SHUTDOWN_TIMEOUT", ""},
	} {
		t.Run(tc.key+"/"+tc.value, func(t *testing.T) {
			setValidEnv(t)
			t.Setenv(tc.key, tc.value)
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("expected error naming %s", tc.key)
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatal("error exposes a secret")
			}
		})
	}
}

func TestRequiredDatabase(t *testing.T) {
	setValidEnv(t)
	t.Setenv("DATABASE_PATH", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected missing database URL error")
	}
}

func TestHTTPSAuthConfig(t *testing.T) {
	setValidEnv(t)
	t.Setenv("BROWSER_ORIGIN", "https://ssh.example.com:8443")
	t.Setenv("SESSION_LIFETIME", "8h")
	t.Setenv("CHALLENGE_LIFETIME", "2m")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RPID != "ssh.example.com" || !cfg.CookieSecure || cfg.SessionLifetime != 8*time.Hour || cfg.ChallengeLifetime != 2*time.Minute {
		t.Fatal("unexpected HTTPS auth configuration")
	}
}
