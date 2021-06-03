package proxy

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"golang.org/x/net/proxy"
)

func init() {
	proxy.RegisterDialerType("tcp", tcptunFromURL)
	proxy.RegisterDialerType("tcptun", tcptunFromURL)
}

func tcptunFromURL(u *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	if u.Port() == "" {
		return nil, fmt.Errorf("proxy/tcptun: missing port in url %v", u)
	}

	return &tcptunDialer{u.Host, forward}, nil
}

type tcptunDialer struct {
	Server  string
	Forward proxy.Dialer
}

func (d *tcptunDialer) Dial(network, addr string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, addr)
}

func (d *tcptunDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, fmt.Errorf("proxy/tcptun: network not implemented: %v", network)
	}

	c, err := Dial(ctx, d.Forward, network, d.Server)
	if err != nil {
		err = fmt.Errorf("proxy/tcptun: dial %v: %w", d.Server, err)
	}

	return c, err
}
