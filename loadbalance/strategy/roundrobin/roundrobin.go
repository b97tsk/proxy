// Package roundrobin provides a load balancing strategy that cyclically picks
// one Dialer out of many.
package roundrobin

import (
	"context"
	"net"
	"sync/atomic"

	"github.com/b97tsk/proxy"
	"github.com/b97tsk/proxy/loadbalance"
)

func init() {
	loadbalance.Register("roundrobin", newDialer)
}

func newDialer(dialers []proxy.Dialer) proxy.Dialer {
	if len(dialers) == 0 {
		panic("proxy/loadbalance/roundrobin: no dialers")
	}

	return &dialer{dialers: dialers}
}

type dialer struct {
	dialers []proxy.Dialer
	index   uint32
}

func (d *dialer) Dial(network, addr string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, addr)
}

func (d *dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	index := atomic.LoadUint32(&d.index)
	for !atomic.CompareAndSwapUint32(&d.index, index, (index+1)%uint32(len(d.dialers))) {
		index = atomic.LoadUint32(&d.index)
	}

	return proxy.Dial(ctx, d.dialers[index], network, addr)
}
