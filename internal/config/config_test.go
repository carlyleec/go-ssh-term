package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	for _, key := range []string{"HTTP_ADDR", "DATABASE_URL", "BROWSER_ORIGIN", "SHUTDOWN_TIMEOUT"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("DATABASE_URL", "postgres://gateway@postgres:5432/gateway")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.BrowserOrigin != "http://127.0.0.1:8080" || cfg.ShutdownTimeout != 5*time.Second {
		t.Fatal("unexpected defaults")
	}
}

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("DATABASE_URL", "postgres://gateway@postgres:5432/gateway")
	t.Setenv("BROWSER_ORIGIN", "http://127.0.0.1:5173")
	t.Setenv("SHUTDOWN_TIMEOUT", "5s")
}

func TestLoad(t *testing.T) {
	setValidEnv(t)
	t.Setenv("HTTP_ADDR", "[::1]:9000")
	t.Setenv("DATABASE_URL", "postgres://gateway:secret@postgres:5432/gateway?sslmode=disable")
	t.Setenv("SHUTDOWN_TIMEOUT", "250ms")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != "[::1]:9000" || cfg.BrowserOrigin != "http://127.0.0.1:5173" || cfg.ShutdownTimeout != 250*time.Millisecond || cfg.DatabaseURL == "" {
		t.Fatalf("unexpected configuration fields")
	}
}

func TestInvalidConfig(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"HTTP_ADDR", "8080"}, {"HTTP_ADDR", ":0"}, {"HTTP_ADDR", ":65536"}, {"HTTP_ADDR", ""},
		{"BROWSER_ORIGIN", "http://localhost:5173/"}, {"BROWSER_ORIGIN", "http://user:secret@localhost"},
		{"BROWSER_ORIGIN", "http://localhost?secret=yes"}, {"BROWSER_ORIGIN", "http://localhost#fragment"},
		{"BROWSER_ORIGIN", "ftp://localhost"}, {"BROWSER_ORIGIN", "http://localhost:99999"}, {"BROWSER_ORIGIN", ""},
		{"DATABASE_URL", "postgres://user:secret%zz@db/app"}, {"DATABASE_URL", "https://db/app"}, {"DATABASE_URL", ""},
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
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected missing database URL error")
	}
}
