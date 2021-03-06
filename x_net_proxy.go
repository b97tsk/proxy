// Code generated by golang.org/x/tools/cmd/bundle. DO NOT EDIT.
//go:generate bundle -o x_net_proxy.go golang.org/x/net/proxy

// Package proxy provides support for a variety of protocols to proxy network
// data.
//

package proxy

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
)

// A ContextDialer dials using a context.
type proxy_ContextDialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// Dial works like DialContext on net.Dialer but using a dialer returned by FromEnvironment.
//
// The passed ctx is only used for returning the Conn, not the lifetime of the Conn.
//
// Custom dialers (registered via RegisterDialerType) that do not implement ContextDialer
// can leak a goroutine for as long as it takes the underlying Dialer implementation to timeout.
//
// A Conn returned from a successful Dial after the context has been cancelled will be immediately closed.
func proxy_Dial(ctx context.Context, network, address string) (net.Conn, error) {
	d := proxy_FromEnvironment()
	if xd, ok := d.(proxy_ContextDialer); ok {
		return xd.DialContext(ctx, network, address)
	}
	return proxy_dialContext(ctx, d, network, address)
}

// WARNING: this can leak a goroutine for as long as the underlying Dialer implementation takes to timeout
// A Conn returned from a successful Dial after the context has been cancelled will be immediately closed.
func proxy_dialContext(ctx context.Context, d proxy_Dialer, network, address string) (net.Conn, error) {
	var (
		conn net.Conn
		done = make(chan struct{}, 1)
		err  error
	)
	go func() {
		conn, err = d.Dial(network, address)
		close(done)
		if conn != nil && ctx.Err() != nil {
			conn.Close()
		}
	}()
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case <-done:
	}
	return conn, err
}

type proxy_direct struct{}

// Direct implements Dialer by making network connections directly using net.Dial or net.DialContext.
var proxy_Direct = proxy_direct{}

var (
	_ proxy_Dialer        = proxy_Direct
	_ proxy_ContextDialer = proxy_Direct
)

// Dial directly invokes net.Dial with the supplied parameters.
func (proxy_direct) Dial(network, addr string) (net.Conn, error) {
	return net.Dial(network, addr)
}

// DialContext instantiates a net.Dialer and invokes its DialContext receiver with the supplied parameters.
func (proxy_direct) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, network, addr)
}

// A PerHost directs connections to a default Dialer unless the host name
// requested matches one of a number of exceptions.
type proxy_PerHost struct {
	def, bypass proxy_Dialer

	bypassNetworks []*net.IPNet
	bypassIPs      []net.IP
	bypassZones    []string
	bypassHosts    []string
}

// NewPerHost returns a PerHost Dialer that directs connections to either
// defaultDialer or bypass, depending on whether the connection matches one of
// the configured rules.
func proxy_NewPerHost(defaultDialer, bypass proxy_Dialer) *proxy_PerHost {
	return &proxy_PerHost{
		def:    defaultDialer,
		bypass: bypass,
	}
}

// Dial connects to the address addr on the given network through either
// defaultDialer or bypass.
func (p *proxy_PerHost) Dial(network, addr string) (c net.Conn, err error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	return p.dialerForRequest(host).Dial(network, addr)
}

// DialContext connects to the address addr on the given network through either
// defaultDialer or bypass.
func (p *proxy_PerHost) DialContext(ctx context.Context, network, addr string) (c net.Conn, err error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	d := p.dialerForRequest(host)
	if x, ok := d.(proxy_ContextDialer); ok {
		return x.DialContext(ctx, network, addr)
	}
	return proxy_dialContext(ctx, d, network, addr)
}

func (p *proxy_PerHost) dialerForRequest(host string) proxy_Dialer {
	if ip := net.ParseIP(host); ip != nil {
		for _, net := range p.bypassNetworks {
			if net.Contains(ip) {
				return p.bypass
			}
		}
		for _, bypassIP := range p.bypassIPs {
			if bypassIP.Equal(ip) {
				return p.bypass
			}
		}
		return p.def
	}

	for _, zone := range p.bypassZones {
		if strings.HasSuffix(host, zone) {
			return p.bypass
		}
		if host == zone[1:] {
			// For a zone ".example.com", we match "example.com"
			// too.
			return p.bypass
		}
	}
	for _, bypassHost := range p.bypassHosts {
		if bypassHost == host {
			return p.bypass
		}
	}
	return p.def
}

// AddFromString parses a string that contains comma-separated values
// specifying hosts that should use the bypass proxy. Each value is either an
// IP address, a CIDR range, a zone (*.example.com) or a host name
// (localhost). A best effort is made to parse the string and errors are
// ignored.
func (p *proxy_PerHost) AddFromString(s string) {
	hosts := strings.Split(s, ",")
	for _, host := range hosts {
		host = strings.TrimSpace(host)
		if len(host) == 0 {
			continue
		}
		if strings.Contains(host, "/") {
			// We assume that it's a CIDR address like 127.0.0.0/8
			if _, net, err := net.ParseCIDR(host); err == nil {
				p.AddNetwork(net)
			}
			continue
		}
		if ip := net.ParseIP(host); ip != nil {
			p.AddIP(ip)
			continue
		}
		if strings.HasPrefix(host, "*.") {
			p.AddZone(host[1:])
			continue
		}
		p.AddHost(host)
	}
}

// AddIP specifies an IP address that will use the bypass proxy. Note that
// this will only take effect if a literal IP address is dialed. A connection
// to a named host will never match an IP.
func (p *proxy_PerHost) AddIP(ip net.IP) {
	p.bypassIPs = append(p.bypassIPs, ip)
}

// AddNetwork specifies an IP range that will use the bypass proxy. Note that
// this will only take effect if a literal IP address is dialed. A connection
// to a named host will never match.
func (p *proxy_PerHost) AddNetwork(net *net.IPNet) {
	p.bypassNetworks = append(p.bypassNetworks, net)
}

// AddZone specifies a DNS suffix that will use the bypass proxy. A zone of
// "example.com" matches "example.com" and all of its subdomains.
func (p *proxy_PerHost) AddZone(zone string) {
	if strings.HasSuffix(zone, ".") {
		zone = zone[:len(zone)-1]
	}
	if !strings.HasPrefix(zone, ".") {
		zone = "." + zone
	}
	p.bypassZones = append(p.bypassZones, zone)
}

// AddHost specifies a host name that will use the bypass proxy.
func (p *proxy_PerHost) AddHost(host string) {
	if strings.HasSuffix(host, ".") {
		host = host[:len(host)-1]
	}
	p.bypassHosts = append(p.bypassHosts, host)
}

// A Dialer is a means to establish a connection.
// Custom dialers should also implement ContextDialer.
type proxy_Dialer interface {
	// Dial connects to the given address via the proxy.
	Dial(network, addr string) (c net.Conn, err error)
}

// Auth contains authentication parameters that specific Dialers may require.
type proxy_Auth struct {
	User, Password string
}

// FromEnvironment returns the dialer specified by the proxy-related
// variables in the environment and makes underlying connections
// directly.
func proxy_FromEnvironment() proxy_Dialer {
	return proxy_FromEnvironmentUsing(proxy_Direct)
}

// FromEnvironmentUsing returns the dialer specify by the proxy-related
// variables in the environment and makes underlying connections
// using the provided forwarding Dialer (for instance, a *net.Dialer
// with desired configuration).
func proxy_FromEnvironmentUsing(forward proxy_Dialer) proxy_Dialer {
	allProxy := proxy_allProxyEnv.Get()
	if len(allProxy) == 0 {
		return forward
	}

	proxyURL, err := url.Parse(allProxy)
	if err != nil {
		return forward
	}
	proxy, err := proxy_FromURL(proxyURL, forward)
	if err != nil {
		return forward
	}

	noProxy := proxy_noProxyEnv.Get()
	if len(noProxy) == 0 {
		return proxy
	}

	perHost := proxy_NewPerHost(proxy, forward)
	perHost.AddFromString(noProxy)
	return perHost
}

// proxySchemes is a map from URL schemes to a function that creates a Dialer
// from a URL with such a scheme.
var proxy_proxySchemes map[string]func(*url.URL, proxy_Dialer) (proxy_Dialer, error)

// RegisterDialerType takes a URL scheme and a function to generate Dialers from
// a URL with that scheme and a forwarding Dialer. Registered schemes are used
// by FromURL.
func proxy_RegisterDialerType(scheme string, f func(*url.URL, proxy_Dialer) (proxy_Dialer, error)) {
	if proxy_proxySchemes == nil {
		proxy_proxySchemes = make(map[string]func(*url.URL, proxy_Dialer) (proxy_Dialer, error))
	}
	proxy_proxySchemes[scheme] = f
}

// FromURL returns a Dialer given a URL specification and an underlying
// Dialer for it to make network requests.
func proxy_FromURL(u *url.URL, forward proxy_Dialer) (proxy_Dialer, error) {
	var auth *proxy_Auth
	if u.User != nil {
		auth = new(proxy_Auth)
		auth.User = u.User.Username()
		if p, ok := u.User.Password(); ok {
			auth.Password = p
		}
	}

	switch u.Scheme {
	case "socks5", "socks5h":
		addr := u.Hostname()
		port := u.Port()
		if port == "" {
			port = "1080"
		}
		return proxy_SOCKS5("tcp", net.JoinHostPort(addr, port), auth, forward)
	}

	// If the scheme doesn't match any of the built-in schemes, see if it
	// was registered by another package.
	if proxy_proxySchemes != nil {
		if f, ok := proxy_proxySchemes[u.Scheme]; ok {
			return f(u, forward)
		}
	}

	return nil, errors.New("proxy: unknown scheme: " + u.Scheme)
}

var (
	proxy_allProxyEnv = &proxy_envOnce{
		names: []string{"ALL_PROXY", "all_proxy"},
	}
	proxy_noProxyEnv = &proxy_envOnce{
		names: []string{"NO_PROXY", "no_proxy"},
	}
)

// envOnce looks up an environment variable (optionally by multiple
// names) once. It mitigates expensive lookups on some platforms
// (e.g. Windows).
// (Borrowed from net/http/transport.go)
type proxy_envOnce struct {
	names []string
	once  sync.Once
	val   string
}

func (e *proxy_envOnce) Get() string {
	e.once.Do(e.init)
	return e.val
}

func (e *proxy_envOnce) init() {
	for _, n := range e.names {
		e.val = os.Getenv(n)
		if e.val != "" {
			return
		}
	}
}

// reset is used by tests
func (e *proxy_envOnce) reset() {
	e.once = sync.Once{}
	e.val = ""
}

// SOCKS5 returns a Dialer that makes SOCKSv5 connections to the given
// address with an optional username and password.
// See RFC 1928 and RFC 1929.
func proxy_SOCKS5(network, address string, auth *proxy_Auth, forward proxy_Dialer) (proxy_Dialer, error) {
	d := socks_NewDialer(network, address)
	if forward != nil {
		if f, ok := forward.(proxy_ContextDialer); ok {
			d.ProxyDial = func(ctx context.Context, network string, address string) (net.Conn, error) {
				return f.DialContext(ctx, network, address)
			}
		} else {
			d.ProxyDial = func(ctx context.Context, network string, address string) (net.Conn, error) {
				return proxy_dialContext(ctx, forward, network, address)
			}
		}
	}
	if auth != nil {
		up := socks_UsernamePassword{
			Username: auth.User,
			Password: auth.Password,
		}
		d.AuthMethods = []socks_AuthMethod{
			socks_AuthMethodNotRequired,
			socks_AuthMethodUsernamePassword,
		}
		d.Authenticate = up.Authenticate
	}
	return d, nil
}
