package connections

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

type terminalInput struct {
	data   []byte
	resize *terminalResize
}

// runTerminal owns this socket and SSH client until the shell or transport ends.
// Input is bounded to one pending message; output uses direct, bounded writes.
func (d *Dialer) runTerminal(parent context.Context, socket *terminalSocket, owner terminalOwner, id string) {
	live, err := d.terminals.start(parent, owner, socket.conn)
	if err != nil {
		terminalSetupFailure(parent, socket, failure(401, "sign in to open a terminal"))
		_ = socket.conn.Close()
		return
	}
	ctx, cancel := live.ctx, live.cancel
	var client *ssh.Client
	var workers sync.WaitGroup
	var audit *terminalAudit
	auditStarted := false
	var finishing atomic.Bool
	defer func() {
		finishing.Store(true)
		d.terminals.release(live)
		cancel()
		_ = socket.conn.Close()
		if client != nil {
			_ = client.Close()
		}
		workers.Wait()
		if auditStarted {
			if live.ioTimedOut.Load() {
				audit.fail("terminal_io_timeout")
			}
			audit.finish()
		}
		d.terminals.complete(live)
	}()
	stopSocket := context.AfterFunc(ctx, func() { _ = socket.conn.Close() })
	defer stopSocket()
	socket.pongWait = d.pongWait
	socket.writeTimeout = d.terminals.ioTimeout
	socket.messageTimeout = d.terminals.ioTimeout
	socket.conn.SetPongHandler(func(string) error { return socket.readDeadline() })
	setupCtx, cancelSetup := context.WithTimeout(ctx, d.timeout)
	defer cancelSetup()
	row, err := d.owned(setupCtx, owner.accountID, id)
	if err != nil {
		terminalSetupFailure(ctx, socket, err)
		return
	}
	jump, routeErr := d.jumpConnection(setupCtx, row)
	audit = &terminalAudit{d: d, row: row, jump: jump, attempt: live.id}
	if err := audit.insert(setupCtx, d.q, "start", ""); err != nil {
		terminalSetupFailure(ctx, socket, failure(503, "could not record connection attempt; try again"))
		return
	}
	auditStarted = true
	setupFailure := func(err error) {
		finishing.Store(true)
		if ctx.Err() == nil {
			err = targetFailure(err)
			var safe *ConnectionErrorBody
			hop := ""
			if errors.As(err, &safe) {
				hop = safe.Hop
			}
			audit.failHop(auditSetupCode(err), hop)
		}
		terminalSetupFailure(ctx, socket, err)
	}
	if routeErr != nil {
		setupFailure(routeErr)
		return
	}
	workers.Go(func() {
		ticker := time.NewTicker(d.pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if socket.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(d.terminals.ioTimeout)) != nil {
					if ctx.Err() == nil {
						audit.fail("browser_transport_failed")
					}
					cancel()
					return
				}
			}
		}
	})
	input := make(chan terminalInput)
	workers.Go(func() {
		defer func() { _ = d.terminals.close(owner, live.id); cancel() }()
		for {
			data, resize, err := socket.read()
			if err != nil {
				if !finishing.Load() && ctx.Err() == nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					audit.fail("browser_transport_failed")
				}
				return
			}
			select {
			case input <- terminalInput{data, resize}:
			case <-ctx.Done():
				return
			}
		}
	})
	if socket.writeStatus("connecting", "") != nil {
		if ctx.Err() == nil {
			audit.fail("browser_transport_failed")
		}
		return
	}
	client, _, err = d.dialRoute(setupCtx, row, jump)
	if err != nil {
		setupFailure(err)
		return
	}
	if err := d.terminals.attach(owner, live.id, client); err != nil {
		return
	}
	stopClient := context.AfterFunc(ctx, func() { _ = client.Close() })
	defer stopClient()
	stopSetup := context.AfterFunc(setupCtx, func() { _ = client.Close() })
	defer stopSetup()
	session, err := client.NewSession()
	if err != nil {
		setupFailure(setupError(setupCtx, failure(502, "could not open SSH terminal")))
		return
	}
	stdin, _ := session.StdinPipe()
	stdout, _ := session.StdoutPipe()
	stderr, _ := session.StderrPipe()
	if err = session.RequestPty("xterm-256color", 24, 80, ssh.TerminalModes{ssh.ECHO: 1}); err == nil {
		err = session.Shell()
	}
	if err != nil {
		setupFailure(setupError(setupCtx, failure(502, "could not start SSH terminal")))
		return
	}
	if !stopSetup() || setupCtx.Err() != nil {
		setupFailure(failure(504, "SSH terminal setup timed out or was canceled"))
		return
	}
	cancelSetup()
	if err := d.terminals.publish(owner, live.id, client, session, stdin); err != nil {
		terminalSetupFailure(ctx, socket, failure(401, "login session is no longer available"))
		return
	}
	if socket.writeStatus("connected", "") != nil {
		if ctx.Err() == nil {
			audit.fail("browser_transport_failed")
		}
		return
	}
	workers.Go(func() {
		ticker := time.NewTicker(d.pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				timer := time.AfterFunc(d.terminals.ioTimeout, func() {
					if ctx.Err() == nil {
						audit.fail("ssh_unresponsive")
					}
					cancel()
				})
				_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
				if !timer.Stop() || err != nil {
					if ctx.Err() == nil {
						audit.fail("ssh_transport_failed")
					}
					cancel()
					return
				}
			}
		}
	})

	ioFailed := make(chan struct{}, 1)
	reportFailure := func() {
		if ctx.Err() == nil {
			audit.fail("terminal_io_failed")
		}
		select {
		case ioFailed <- struct{}{}:
		default:
		}
	}
	var output sync.WaitGroup
	for _, stream := range []io.Reader{stdout, stderr} {
		output.Add(1)
		workers.Go(func() {
			defer output.Done()
			if _, err := io.Copy(terminalOutput{socket: socket, cancel: func() { reportFailure(); cancel() }}, stream); err != nil {
				reportFailure()
			}
		})
	}
	workers.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-input:
				var err error
				if event.resize != nil {
					err = d.terminals.resize(owner, live.id, event.resize.Rows, event.resize.Cols)
				} else {
					err = d.terminals.input(owner, live.id, event.data)
				}
				if err != nil {
					reportFailure()
					return
				}
			}
		}
	})
	wait := make(chan error, 1)
	workers.Go(func() {
		err := session.Wait()
		// Drain both SSH streams before publishing the final status.
		timer := time.AfterFunc(d.terminals.ioTimeout, func() {
			if ctx.Err() == nil {
				audit.fail("output_drain_timeout")
			}
			cancel()
		})
		output.Wait()
		timer.Stop()
		wait <- err
	})
	message := "SSH connection ended."
	select {
	case <-ctx.Done():
		return
	case <-ioFailed:
		_ = client.Close()
		output.Wait()
		message = "SSH terminal I/O failed."
	case err := <-wait:
		if err == nil {
			message = "Shell exited."
		} else {
			audit.fail("ssh_session_failed")
		}
	}
	finishing.Store(true)
	_ = socket.writeStatus("disconnected", message)
	socket.close(websocket.CloseNormalClosure, "terminal ended")
}

func terminalSetupFailure(ctx context.Context, socket *terminalSocket, err error) {
	if ctx.Err() != nil {
		return
	}
	message := "could not start SSH terminal"
	var safe *ConnectionErrorBody
	if errors.As(err, &safe) {
		message = connectionErrorMessage(safe)
	}
	_ = socket.writeStatus("failed", message)
	socket.close(websocket.CloseNormalClosure, "terminal setup failed")
}

type terminalOutput struct {
	socket *terminalSocket
	cancel context.CancelFunc
}

func (w terminalOutput) Write(data []byte) (int, error) {
	written := 0
	for len(data) > 0 {
		size := min(len(data), terminalDataLimit)
		if err := w.socket.writeData(data[:size]); err != nil {
			w.cancel()
			return written, err
		}
		written += size
		data = data[size:]
	}
	return written, nil
}
