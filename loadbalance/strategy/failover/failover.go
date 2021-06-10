// Package failover provides a load balancing strategy that it tries to pick
// a reliable Dialer out of many.
package failover

import (
	"container/heap"
	"context"
	"net"
	"sync"

	"github.com/b97tsk/proxy"
	"github.com/b97tsk/proxy/loadbalance"
)

func init() {
	loadbalance.Register("failover", newDialer)
}

func newDialer(dialers []proxy.Dialer) proxy.Dialer {
	if len(dialers) == 0 {
		panic("proxy/loadbalance/failover: no dialers")
	}

	heap := make(dialerHeap, len(dialers))

	for i := range heap {
		heap[i] = &dialerItem{
			Dialer:    dialers[i],
			HeapIndex: i,
			SeqIndex:  i,
			Score:     maxScore / 2,
		}
	}

	return &dialer{dialers: heap}
}

type dialer struct {
	mu      sync.Mutex
	dialers dialerHeap
	numLow  int // number of dialers that have low score (lower than maxScore/2)
	numHigh int // number of dialers that have high score (higher than maxScore/2)
}

func (d *dialer) Dial(network, addr string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, addr)
}

func (d *dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	d.mu.Lock()
	t := d.dialers[0]
	d.mu.Unlock()

	c, err := proxy.Dial(ctx, t.Dialer, network, addr)

	d.mu.Lock()
	d.fix(t, err == nil)
	d.mu.Unlock()

	return c, err
}

func (d *dialer) fix(t *dialerItem, success bool) {
	oldScore := t.Score

	if success {
		t.Success()
	} else {
		t.Failure()
	}

	d.scoreChanged(oldScore, t.Score)

	heap.Fix(&d.dialers, t.HeapIndex)

	n := len(d.dialers)
	if d.numLow < n || d.numHigh < n {
		return
	}

	totalScore := 0

	for _, t := range d.dialers {
		totalScore += t.Score
	}

	offset := maxScore/2 - totalScore/n

	for _, t := range d.dialers {
		oldScore := t.Score
		t.Score += offset
		d.scoreChanged(oldScore, t.Score)
	}
}

func (d *dialer) scoreChanged(oldScore, newScore int) {
	switch {
	case oldScore < maxScore/2:
		switch {
		case newScore > maxScore/2:
			d.numLow--
			d.numHigh++
		case newScore == maxScore/2:
			d.numLow--
		}
	case oldScore > maxScore/2:
		switch {
		case newScore < maxScore/2:
			d.numLow++
			d.numHigh--
		case newScore == maxScore/2:
			d.numHigh--
		}
	default:
		switch {
		case newScore < maxScore/2:
			d.numLow++
		case newScore > maxScore/2:
			d.numHigh++
		}
	}
}

type dialerHeap []*dialerItem

func (h dialerHeap) Len() int           { return len(h) }
func (h dialerHeap) Less(i, j int) bool { return h[i].Less(h[j]) }

func (h dialerHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].HeapIndex = i
	h[j].HeapIndex = j
}

func (h *dialerHeap) Push(x interface{}) {
	t := x.(*dialerItem)
	t.HeapIndex = len(*h)
	*h = append(*h, t)
}

func (h *dialerHeap) Pop() interface{} {
	old := *h
	n := len(old)
	t := old[n-1]
	old[n-1] = nil
	t.HeapIndex = -1
	*h = old[0 : n-1]

	return t
}

type dialerItem struct {
	Dialer    proxy.Dialer
	HeapIndex int
	SeqIndex  int
	Score     int
	N         int // number of consecutive successes or failures
}

func (t *dialerItem) Less(other *dialerItem) bool {
	return t.Score > other.Score || t.SeqIndex < other.SeqIndex
}

func (t *dialerItem) Success() {
	if t.Score == maxScore {
		return
	}

	if t.N < 0 {
		t.N = 0
	}

	t.N++

	t.Score += fibonacci(t.N)
	if t.Score > maxScore {
		t.Score = maxScore
	}
}

func (t *dialerItem) Failure() {
	if t.Score == 0 {
		return
	}

	if t.N > 0 {
		t.N = 0
	}

	t.N--

	t.Score -= fibonacci(-t.N)
	if t.Score < 0 {
		t.Score = 0
	}
}

func fibonacci(n int) int {
	for i, j := 0, 1; ; i, j, n = j, i+j, n-1 { //nolint:wastedassign
		if n < 1 {
			return i
		}
	}
}

const maxScore = 100
