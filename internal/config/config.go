package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	BrowserOrigin   string
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:      value("HTTP_ADDR", ":8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		BrowserOrigin: value("BROWSER_ORIGIN", "http://127.0.0.1:8080"),
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
	if cfg.DatabaseURL != "" {
		database, err := url.Parse(cfg.DatabaseURL)
		if err != nil || (database.Scheme != "postgres" && database.Scheme != "postgresql") || database.Hostname() == "" {
			return Config{}, fmt.Errorf("DATABASE_URL must be a postgres:// or postgresql:// URL with a host")
		}
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
