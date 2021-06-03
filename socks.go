package proxy

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"golang.org/x/net/proxy"
)

func init() {
	proxy.RegisterDialerType("socks", socksFromURL)
}

func socksFromURL(u *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	var auth *proxy.Auth
	if u.User != nil {
		auth = new(proxy.Auth)
		auth.User = u.User.Username()

		if p, ok := u.User.Password(); ok {
			auth.Password = p
		}
	}

	d, err := proxy.SOCKS5("tcp", u.Host, auth, forward)
	if err != nil {
		return nil, fmt.Errorf("proxy/socks: %w", err)
	}

	return &socksDialer{d}, nil
}

type socksDialer struct {
	Forward proxy.Dialer
}

func (d *socksDialer) Dial(network, addr string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, addr)
}

func (d *socksDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, fmt.Errorf("proxy/socks: network not implemented: %v", network)
	}

	c, err := Dial(ctx, d.Forward, network, addr)
	if err != nil {
		err = fmt.Errorf("proxy/socks: dial %v: %w", addr, err)
	}

	return c, err
}
