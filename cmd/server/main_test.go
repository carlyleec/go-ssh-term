package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestShutdown(t *testing.T) {
	for _, force := range []bool{false, true} {
		name := "drain"
		if force {
			name = "deadline"
		}
		t.Run(name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			entered := make(chan struct{})
			release := make(chan struct{})
			defer close(release)
			server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(entered)
				select {
				case <-release:
				case <-r.Context().Done():
				}
			})}
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			finished := make(chan error, 1)
			timeout := 2 * time.Second
			if force {
				timeout = 50 * time.Millisecond
			}
			go func() { finished <- serve(ctx, server, listener, timeout) }()
			client := &http.Client{Timeout: 3 * time.Second}
			requestDone := make(chan error, 1)
			go func() {
				resp, err := client.Get("http://" + listener.Addr().String())
				if resp != nil {
					resp.Body.Close()
				}
				requestDone <- err
			}()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("request did not start")
			}
			cancel()
			if !force {
				release <- struct{}{}
			}
			select {
			case err := <-finished:
				if force && !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("expected deadline, got %v", err)
				}
				if !force && err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("shutdown did not finish")
			}
			if err := <-requestDone; !force && err != nil {
				t.Fatal(err)
			}
		})
	}
}
