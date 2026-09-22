package docker

import (
	"context"
	"net"
	"sync"
	"time"
)

// serialSSHDialer bounds unauthenticated connections per host. Docker's SSH
// helper returns after starting the subprocess, before the master socket is
// ready. Hold the gate until a response proves the connection is established,
// or until it fails/closes. Existing streams never hold the gate.
func serialSSHDialer(dial func(context.Context, string, string) (net.Conn, error)) func(context.Context, string, string) (net.Conn, error) {
	gate := make(chan struct{}, 1)
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		select {
		case gate <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		if err := ctx.Err(); err != nil {
			<-gate
			return nil, err
		}
		conn, err := dial(ctx, network, addr)
		if err != nil {
			<-gate
			return nil, err
		}
		release := sync.OnceFunc(func() { <-gate })
		// A speculative HTTP dial may enter the idle pool without ever being
		// used. Bound that wait too, since no response-header timer runs yet.
		timer := time.AfterFunc(sshHandshakeTimeout, func() {
			_ = conn.Close()
			release()
		})
		return &sshHandshakeConn{Conn: conn, release: func() {
			timer.Stop()
			release()
		}}, nil
	}
}

const sshHandshakeTimeout = 10 * time.Second

type sshHandshakeConn struct {
	net.Conn
	release func()
}

func (c *sshHandshakeConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 || err != nil {
		c.release()
	}
	return n, err
}

func (c *sshHandshakeConn) Close() error {
	err := c.Conn.Close()
	c.release()
	return err
}

// Preserve the half-close operation used by Docker's hijacked connections.
func (c *sshHandshakeConn) CloseWrite() error {
	if conn, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return conn.CloseWrite()
	}
	return nil
}
