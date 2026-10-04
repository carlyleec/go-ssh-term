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
func (d *Dialer) runTerminal(parent context.Context, socket *terminalSocket, accountID, id string) {
	ctx, cancel := context.WithCancel(parent)
	var client *ssh.Client
	var workers sync.WaitGroup
	defer func() {
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
		defer cancel()
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
	var err error
	client, _, err = d.Dial(setupCtx, accountID, id)
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
					err = session.WindowChange(event.resize.Rows, event.resize.Cols)
				} else {
					_, err = stdin.Write(event.data)
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
