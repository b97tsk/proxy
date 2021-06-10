// Package random provides a load balancing strategy that randomly picks
// one Dialer out of many.
package random

import (
	"context"
	"math/rand"
	"net"

	"github.com/b97tsk/proxy"
	"github.com/b97tsk/proxy/loadbalance"
)

func init() {
	loadbalance.Register("random", newDialer)
}

func newDialer(dialers []proxy.Dialer) proxy.Dialer {
	if len(dialers) == 0 {
		panic("proxy/loadbalance/random: no dialers")
	}

	return &dialer{dialers: dialers}
}

type dialer struct {
	dialers []proxy.Dialer
}

func (d *dialer) Dial(network, addr string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, addr)
}

func (d *dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return proxy.Dial(ctx, d.dialers[rand.Intn(len(d.dialers))], network, addr)
}
