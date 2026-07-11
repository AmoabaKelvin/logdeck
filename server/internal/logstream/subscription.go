package logstream

import (
	"log"
	"sync"
	"sync/atomic"

	"github.com/AmoabaKelvin/logdeck/internal/models"
)

const (
	// ringSize bounds each subscription's record buffer; on overflow the
	// oldest record is dropped so tails never block on a slow sink.
	ringSize = 1024
	// dropLogEvery throttles overflow logging to one line per N drops.
	dropLogEvery = 1000
)

// subscription pairs one subscriber's spec, options, and sink with its tails
// and its bounded delivery buffer. Records are pushed by tail goroutines and
// consumed by a single delivery goroutine that invokes sink sequentially.
type subscription struct {
	spec ContainerSpec
	opts models.LogOptions
	sink func(Record)

	// tails is owned exclusively by the hub's run loop goroutine.
	tails map[containerKey]*tail

	mu      sync.Mutex
	cond    *sync.Cond
	buf     []Record // fixed-size ring
	head    int
	count   int
	closed  bool
	discard bool // when closed: drop buffered records instead of draining

	drops     atomic.Uint64
	delivered chan struct{} // closed when the delivery goroutine exits
}

func newSubscription(spec ContainerSpec, opts models.LogOptions, sink func(Record)) *subscription {
	s := &subscription{
		spec:      spec,
		opts:      opts,
		sink:      sink,
		tails:     make(map[containerKey]*tail),
		buf:       make([]Record, ringSize),
		delivered: make(chan struct{}),
	}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// push enqueues a record, dropping the oldest buffered record on overflow.
// It never blocks on the sink. Records pushed after close are discarded.
func (s *subscription) push(r Record) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	dropped := false
	if s.count == len(s.buf) {
		s.buf[s.head] = Record{}
		s.head = (s.head + 1) % len(s.buf)
		s.count--
		dropped = true
	}
	s.buf[(s.head+s.count)%len(s.buf)] = r
	s.count++
	s.cond.Signal()
	s.mu.Unlock()

	if dropped {
		if n := s.drops.Add(1); n%dropLogEvery == 0 {
			log.Printf("logstream: subscription %+v dropped %d records (slow sink)", s.spec, n)
		}
	}
}

// deliverLoop invokes the sink sequentially until the subscription is closed.
// A drain close finishes buffered records first; a discard close returns
// immediately without further sink calls.
func (s *subscription) deliverLoop() {
	defer close(s.delivered)
	for {
		s.mu.Lock()
		for s.count == 0 && !s.closed {
			s.cond.Wait()
		}
		if s.closed && (s.discard || s.count == 0) {
			s.mu.Unlock()
			return
		}
		r := s.buf[s.head]
		s.buf[s.head] = Record{}
		s.head = (s.head + 1) % len(s.buf)
		s.count--
		s.mu.Unlock()
		s.sink(r)
	}
}

// close stops the subscription; the first call wins (a later close cannot
// change drain/discard semantics). Idempotent.
func (s *subscription) close(discard bool) {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		s.discard = discard
		s.cond.Broadcast()
	}
	s.mu.Unlock()
}
