// Package loadbalance provides load balancing over Dialers.
package loadbalance

import (
	"github.com/b97tsk/proxy"
)

// A Strategy is a function that accepts some Dialers as arguments and returns
// a Dialer that when dials, it picks one from those provided Dialers with
// certain strategy, and completes the dialing with this selected Dialer.
type Strategy func([]proxy.Dialer) proxy.Dialer

// Get gets a registered Strategy by name.
// Get returns nil if there is no Strategy registered under this name.
func Get(name string) Strategy {
	return strategies[name]
}

// Register registers a Strategy under specified name.
func Register(name string, s Strategy) {
	if s == nil {
		panic("proxy/loadbalance: nil Strategy")
	}

	if strategies == nil {
		strategies = make(map[string]Strategy)
	}

	strategies[name] = s
}

var strategies map[string]Strategy
