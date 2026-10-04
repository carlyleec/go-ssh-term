package main

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestShutdownJoinsHijackedConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan net.Conn, 1)
	handlerDone := make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(handlerDone)
		conn, buffer, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		_, _ = buffer.WriteString("ready\n")
		_ = buffer.Flush()
		accepted <- conn
		_, _ = buffer.ReadByte()
	})}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, server, listener, time.Second, func(shutdown context.Context) error {
			select {
			case conn := <-accepted:
				_ = conn.Close()
			case <-shutdown.Done():
				return shutdown.Err()
			}
			<-handlerDone
			return nil
		})
	}()
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	_, _ = conn.Write([]byte("GET / HTTP/1.1\r\nHost: localhost\r\n\r\n"))
	if _, err := bufio.NewReader(conn).ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown hung")
	}
	select {
	case <-handlerDone:
	default:
		t.Fatal("hijacked handler survived shutdown")
	}
}

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
			go func() { finished <- serve(ctx, server, listener, timeout, nil) }()
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
