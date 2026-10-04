package connections

import (
	"context"
	"errors"
	"io"
	"sync"

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
	live, err := d.terminals.start(parent, owner)
	if err != nil {
		terminalSetupFailure(parent, socket, failure(401, "sign in to open a terminal"))
		_ = socket.conn.Close()
		return
	}
	ctx, cancel := live.ctx, live.cancel
	var client *ssh.Client
	var workers sync.WaitGroup
	defer func() {
		d.terminals.release(live)
		cancel()
		_ = socket.conn.Close()
		if client != nil {
			_ = client.Close()
		}
		workers.Wait()
	}()
	stopSocket := context.AfterFunc(ctx, func() { _ = socket.conn.Close() })
	defer stopSocket()
	input := make(chan terminalInput)
	workers.Go(func() {
		defer func() { _ = d.terminals.close(owner, live.id); cancel() }()
		for {
			data, resize, err := socket.read()
			if err != nil {
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
		return
	}
	setupCtx, cancelSetup := context.WithTimeout(ctx, d.timeout)
	defer cancelSetup()
	client, _, err = d.Dial(setupCtx, owner.accountID, id)
	if err != nil {
		terminalSetupFailure(ctx, socket, err)
		return
	}
	stopClient := context.AfterFunc(ctx, func() { _ = client.Close() })
	defer stopClient()
	stopSetup := context.AfterFunc(setupCtx, func() { _ = client.Close() })
	defer stopSetup()
	session, err := client.NewSession()
	if err != nil {
		terminalSetupFailure(ctx, socket, setupError(setupCtx, failure(502, "could not open SSH terminal")))
		return
	}
	stdin, _ := session.StdinPipe()
	stdout, _ := session.StdoutPipe()
	stderr, _ := session.StderrPipe()
	if err = session.RequestPty("xterm-256color", 24, 80, ssh.TerminalModes{ssh.ECHO: 1}); err == nil {
		err = session.Shell()
	}
	if err != nil {
		terminalSetupFailure(ctx, socket, setupError(setupCtx, failure(502, "could not start SSH terminal")))
		return
	}
	if !stopSetup() || setupCtx.Err() != nil {
		terminalSetupFailure(ctx, socket, failure(504, "SSH terminal setup timed out or was canceled"))
		return
	}
	cancelSetup()
	if err := d.terminals.publish(owner, live.id, client, session, stdin); err != nil {
		terminalSetupFailure(ctx, socket, failure(401, "login session is no longer available"))
		return
	}
	if socket.writeStatus("connected", "") != nil {
		return
	}

	ioFailed := make(chan struct{}, 1)
	reportFailure := func() {
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
			if _, err := io.Copy(terminalOutput{socket}, stream); err != nil {
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
		output.Wait()
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
		}
	}
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
		message = safe.Message
	}
	_ = socket.writeStatus("failed", message)
	socket.close(websocket.CloseNormalClosure, "terminal setup failed")
}

type terminalOutput struct{ socket *terminalSocket }

func (w terminalOutput) Write(data []byte) (int, error) {
	written := 0
	for len(data) > 0 {
		size := min(len(data), terminalDataLimit)
		if err := w.socket.writeData(data[:size]); err != nil {
			return written, err
		}
		written += size
		data = data[size:]
	}
	return written, nil
}
