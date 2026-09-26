package docker

import (
	"bytes"
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// maxExecOutput caps how many bytes RunExec keeps of each output stream, so a
// runaway command can't exhaust memory.
const maxExecOutput = 256 * 1024

// capWriter keeps the first maxExecOutput bytes written to it and drops the
// rest, so the command can still run to completion.
type capWriter struct {
	buf       bytes.Buffer
	truncated bool
}

func (w *capWriter) Write(p []byte) (int, error) {
	room := maxExecOutput - w.buf.Len()
	if len(p) > room {
		w.truncated = true
		w.buf.Write(p[:max(room, 0)])
		return len(p), nil
	}
	return w.buf.Write(p)
}

// CreateExec creates an exec instance for a container, launching /bin/bash
// and falling back to /bin/sh if bash is not available.
func (c *MultiHostClient) CreateExec(ctx context.Context, host, containerID string) (string, error) {
	cli, err := c.GetClient(host)
	if err != nil {
		return "", err
	}

	cmd := []string{"/bin/sh", "-c", "(test -x /bin/bash && exec /bin/bash) || exec /bin/sh"}

	config := container.ExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          cmd,
	}

	response, err := cli.ContainerExecCreate(ctx, containerID, config)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	return response.ID, nil
}

func (c *MultiHostClient) AttachExec(ctx context.Context, host, execID string) (*types.HijackedResponse, error) {
	cli, err := c.GetClient(host)
	if err != nil {
		return nil, err
	}

	resp, err := cli.ContainerExecAttach(ctx, execID, container.ExecStartOptions{
		Tty: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach to exec: %w", err)
	}

	return &resp, nil
}

func (c *MultiHostClient) ResizeExec(ctx context.Context, host, execID string, height, width uint) error {
	cli, err := c.GetClient(host)
	if err != nil {
		return err
	}

	return cli.ContainerExecResize(ctx, execID, container.ResizeOptions{
		Height: height,
		Width:  width,
	})
}

// RunExec runs one command in a container and returns its separated stdout and
// stderr and its exit code. Unlike CreateExec (the interactive terminal) it
// allocates no TTY, so the two streams stay distinct and the exit code comes
// from the daemon rather than being scraped from shell output. The command runs
// to completion; the caller bounds it with ctx. Each stream keeps at most
// maxExecOutput bytes, and truncated reports when either was cut.
func (c *MultiHostClient) RunExec(ctx context.Context, host, containerID string, cmd []string) (stdout, stderr string, exitCode int, truncated bool, err error) {
	cli, err := c.GetClient(host)
	if err != nil {
		return "", "", 0, false, err
	}

	created, err := cli.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	})
	if err != nil {
		return "", "", 0, false, fmt.Errorf("failed to create exec: %w", err)
	}

	attach, err := cli.ContainerExecAttach(ctx, created.ID, container.ExecStartOptions{})
	if err != nil {
		return "", "", 0, false, fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer attach.Close()

	// Without a TTY the stream is multiplexed; stdcopy demultiplexes it back into
	// the two streams. Copy on a goroutine so a hung command releases on ctx:
	// closing the hijacked connection unblocks the blocked read.
	var outBuf, errBuf capWriter
	done := make(chan error, 1)
	go func() {
		_, copyErr := stdcopy.StdCopy(&outBuf, &errBuf, attach.Reader)
		done <- copyErr
	}()
	select {
	case copyErr := <-done:
		if copyErr != nil {
			return "", "", 0, false, fmt.Errorf("failed to read exec output: %w", copyErr)
		}
	case <-ctx.Done():
		attach.Close()
		return "", "", 0, false, ctx.Err()
	}

	inspect, err := cli.ContainerExecInspect(ctx, created.ID)
	if err != nil {
		return "", "", 0, false, fmt.Errorf("failed to inspect exec: %w", err)
	}
	return outBuf.buf.String(), errBuf.buf.String(), inspect.ExitCode, outBuf.truncated || errBuf.truncated, nil
}
