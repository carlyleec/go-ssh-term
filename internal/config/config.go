package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/database/sqlite"
	"github.com/carlyleec/go-ssh-term/internal/sshkeys"
)

type Config struct {
	HTTPAddr          string
	DatabasePath      string
	EncryptionKeyPath string
	BrowserOrigin     string
	ShutdownTimeout   time.Duration
	RPID              string
	CookieSecure      bool
	SessionLifetime   time.Duration
	ChallengeLifetime time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:          value("HTTP_ADDR", ":8080"),
		DatabasePath:      os.Getenv("DATABASE_PATH"),
		EncryptionKeyPath: os.Getenv("ENCRYPTION_KEY_PATH"),
		BrowserOrigin:     value("BROWSER_ORIGIN", "http://localhost:8080"),
	}
	_, port, err := net.SplitHostPort(cfg.HTTPAddr)
	if err != nil || !validPort(port) {
		return Config{}, fmt.Errorf("HTTP_ADDR must be a host:port address with a port from 1 to 65535")
	}
	origin, err := url.Parse(cfg.BrowserOrigin)
	if err != nil || origin.Hostname() == "" || (origin.Scheme != "http" && origin.Scheme != "https") || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.ForceQuery || origin.Fragment != "" {
		return Config{}, fmt.Errorf("BROWSER_ORIGIN must be an http(s) origin with no credentials, path, query, or fragment")
	}
	if origin.Port() != "" && !validPort(origin.Port()) {
		return Config{}, fmt.Errorf("BROWSER_ORIGIN port must be from 1 to 65535")
	}
	if net.ParseIP(origin.Hostname()) != nil || (origin.Scheme == "http" && origin.Hostname() != "localhost") {
		return Config{}, fmt.Errorf("BROWSER_ORIGIN must use localhost for HTTP or a domain name over HTTPS for WebAuthn")
	}
	cfg.RPID = origin.Hostname()
	cfg.CookieSecure = origin.Scheme == "https"
	cfg.SessionLifetime, err = time.ParseDuration(value("SESSION_LIFETIME", "12h"))
	if err != nil || cfg.SessionLifetime < time.Second {
		return Config{}, fmt.Errorf("SESSION_LIFETIME must be a duration of at least 1s")
	}
	cfg.ChallengeLifetime, err = time.ParseDuration(value("CHALLENGE_LIFETIME", "5m"))
	if err != nil || cfg.ChallengeLifetime < time.Millisecond {
		return Config{}, fmt.Errorf("CHALLENGE_LIFETIME must be a duration of at least 1ms")
	}
	if err := sqlite.ValidatePath(cfg.DatabasePath); err != nil {
		return Config{}, err
	}
	if err := sshkeys.ValidateEncryptionKeyPath(cfg.EncryptionKeyPath); err != nil {
		return Config{}, err
	}
	if filepath.Dir(cfg.EncryptionKeyPath) == filepath.Dir(cfg.DatabasePath) {
		return Config{}, fmt.Errorf("ENCRYPTION_KEY_PATH must use a separate directory from DATABASE_PATH")
	}
	cfg.ShutdownTimeout, err = time.ParseDuration(value("SHUTDOWN_TIMEOUT", "5s"))
	if err != nil || cfg.ShutdownTimeout <= 0 {
		return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT must be a positive duration such as 5s")
	}
	return cfg, nil
}

func value(name, fallback string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}
	return fallback
}

func validPort(port string) bool {
	n, err := strconv.Atoi(port)
	return err == nil && n >= 1 && n <= 65535
}
