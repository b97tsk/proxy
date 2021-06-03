package proxy

import (
	"net"
	"net/url"
)

type Dialer interface {
	Dial(network, addr string) (net.Conn, error)
}

var Direct Dialer = proxy_Direct

func FromEnvironment() Dialer {
	return proxy_FromEnvironmentUsing(Direct)
}

func FromEnvironmentUsing(forward Dialer) Dialer {
	return proxy_FromEnvironmentUsing(forward)
}

func RegisterDialerType(scheme string, f func(*url.URL, Dialer) (Dialer, error)) {
	proxy_RegisterDialerType(scheme, func(u *url.URL, fwd proxy_Dialer) (proxy_Dialer, error) {
		return f(u, fwd)
	})
}

func FromURL(u *url.URL, forward Dialer) (Dialer, error) {
	switch u.Scheme {
	case "socks5", "socks5h":
		u2 := *u
		u = &u2
		u.Scheme = "socks"
	}

	return proxy_FromURL(u, forward)
}
