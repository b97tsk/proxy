package proxy

import (
	"context"
	"net"
)

type ContextDialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

func Dial(ctx context.Context, d Dialer, network, addr string) (net.Conn, error) {
	if xd, ok := d.(ContextDialer); ok {
		return xd.DialContext(ctx, network, addr)
	}

	return dialFallback(ctx, d, network, addr)
}

func dialFallback(ctx context.Context, d Dialer, network, addr string) (net.Conn, error) {
	done := make(chan struct{})
	defer close(done)

	type Result struct {
		Conn net.Conn
		Err  error
	}

	result := make(chan Result, 1)

	go func() {
		c, err := d.Dial(network, addr)
		result <- Result{c, err}

		if <-done; len(result) == 1 {
			if c != nil {
				_ = c.Close()
			}
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-result:
		return r.Conn, r.Err
	}
}
