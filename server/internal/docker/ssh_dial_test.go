package docker

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func TestSSHDialsWaitForHandshake(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var started atomic.Int32
		peers := make(chan net.Conn, 45)
		dial := serialSSHDialer(func(context.Context, string, string) (net.Conn, error) {
			started.Add(1)
			client, peer := net.Pipe()
			peers <- peer
			return client, nil
		})
		connections := make(chan net.Conn, 45)
		var wg sync.WaitGroup
		for range 45 {
			wg.Go(func() {
				conn, err := dial(context.Background(), "tcp", "docker")
				if err != nil {
					t.Error(err)
					return
				}
				connections <- conn
			})
		}
		var opened []net.Conn
		for i := range 45 {
			conn := <-connections
			peer := <-peers
			synctest.Wait()
			if got := started.Load(); got != int32(i+1) {
				t.Fatalf("started %d SSH processes before handshake %d completed", got, i+1)
			}
			go func() { _, _ = peer.Write([]byte("H")) }()
			if _, err := io.ReadFull(conn, make([]byte, 1)); err != nil {
				t.Fatal(err)
			}
			// Keep every established stream open while subsequent dials run.
			opened = append(opened, conn, peer)
		}
		wg.Wait()
		// The handshake timeout must never cut off a followed stream later.
		time.Sleep(2 * sshHandshakeTimeout)
		go func() { _, _ = opened[1].Write([]byte("L")) }()
		if _, err := io.ReadFull(opened[0], make([]byte, 1)); err != nil {
			t.Fatalf("established stream timed out: %v", err)
		}
		for _, conn := range opened {
			_ = conn.Close()
		}
	})
}

func TestSSHDialCancellationAndFailures(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		wantErr := errors.New("cannot start ssh")
		attempts := 0
		dial := serialSSHDialer(func(context.Context, string, string) (net.Conn, error) {
			attempts++
			if attempts == 1 {
				return nil, wantErr
			}
			client, peer := net.Pipe()
			t.Cleanup(func() { _ = peer.Close() })
			return client, nil
		})
		if _, err := dial(context.Background(), "", ""); !errors.Is(err, wantErr) {
			t.Fatalf("dial error = %v", err)
		}
		first, err := dial(context.Background(), "", "")
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if _, err := dial(ctx, "", ""); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("waiting dial error = %v", err)
		}
		if attempts != 2 {
			t.Fatalf("canceled waiter started an SSH process: attempts=%d", attempts)
		}
		_ = first.Close()
		second, err := dial(context.Background(), "", "")
		if err != nil {
			t.Fatal(err)
		}
		_ = second.Close()
	})
}

func TestSSHUnusedDialTimesOut(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dial := serialSSHDialer(func(context.Context, string, string) (net.Conn, error) {
			client, peer := net.Pipe()
			t.Cleanup(func() { _ = peer.Close() })
			return client, nil
		})
		first, err := dial(context.Background(), "", "")
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		second, err := dial(context.Background(), "", "")
		if err != nil {
			t.Fatal(err)
		}
		if elapsed := time.Since(start); elapsed != sshHandshakeTimeout {
			t.Fatalf("unused connection held gate for %s", elapsed)
		}
		_ = first.Close()
		_ = second.Close()
	})
}
