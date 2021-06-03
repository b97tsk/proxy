package proxy

import (
	"net/url"

	"golang.org/x/net/proxy"
)

type (
	Dialer        = proxy.Dialer
	ContextDialer = proxy.ContextDialer
)

var Direct Dialer = proxy.Direct

func FromURL(u *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	switch u.Scheme {
	case "socks5", "socks5h":
		u2 := *u
		u = &u2
		u.Scheme = "socks"
	}

	return proxy.FromURL(u, forward)
}
