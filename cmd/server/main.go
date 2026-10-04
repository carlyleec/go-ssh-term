package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/carlyleec/go-ssh-term/internal/api"
	"github.com/carlyleec/go-ssh-term/internal/auth"
	"github.com/carlyleec/go-ssh-term/internal/config"
	"github.com/carlyleec/go-ssh-term/internal/database"
	"github.com/carlyleec/go-ssh-term/internal/sshkeys"
	"github.com/carlyleec/go-ssh-term/internal/web"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	wa, err := auth.NewWebAuthn(cfg)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := database.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer pool.Close()
	encryption, err := sshkeys.OpenEncryption(ctx, pool, cfg.EncryptionKeyPath)
	if err != nil {
		return err
	}
	sessions, stopCleanup := auth.NewSessions(cfg, pool)
	defer stopCleanup()
	mux := http.NewServeMux()
	access := auth.NewAccess(sessions, pool, cfg.RPID, cfg.BrowserOrigin)
	keys := sshkeys.NewHandler(pool, encryption, access)
	mux.Handle("/api/keys", keys)
	mux.Handle("/api/keys/", keys)
	contract := api.New(mux)
	access.RegisterCurrentUser(contract)
	access.RegisterLogout(contract)
	mux.Handle("/api/auth/register/", auth.NewRegistration(wa, sessions, pool, cfg.BrowserOrigin))
	mux.Handle("/api/auth/login/", auth.NewLogin(wa, sessions, pool, cfg.BrowserOrigin))
	mux.Handle("GET /api/readyz", readiness(func(ctx context.Context) error { return database.Check(ctx, pool) }, os.DirFS("frontend/dist")))
	mux.Handle("/", web.Handler(os.DirFS("frontend/dist")))
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: mux}
	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return err
	}
	log.Printf("Listening on %s", listener.Addr())
	return serve(ctx, server, listener, cfg.ShutdownTimeout)
}

func serve(ctx context.Context, server *http.Server, listener net.Listener, timeout time.Duration) error {
	result := make(chan error, 1)
	go func() { result <- server.Serve(listener) }()
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return err
		}
		return nil
	}
}
