package khatru

import (
	"container/heap"
	"context"
	"iter"
	"slices"
	"sync"
	"time"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip40"
)

type expiringEvent struct {
	id        nostr.ID
	expiresAt nostr.Timestamp
}

type expiringEventHeap []expiringEvent

func (h expiringEventHeap) Len() int           { return len(h) }
func (h expiringEventHeap) Less(i, j int) bool { return h[i].expiresAt < h[j].expiresAt }
func (h expiringEventHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *expiringEventHeap) Push(x interface{}) {
	*h = append(*h, x.(expiringEvent))
}

func (h *expiringEventHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type expirationManager struct {
	events expiringEventHeap
	mu     sync.Mutex

	queryStored func(ctx context.Context, filter nostr.Filter) iter.Seq[nostr.Event]
	deleteEvent func(ctx context.Context, id nostr.ID) error

	interval        time.Duration
	initialScanDone bool
	kill            chan struct{} // used for manually killing this
	killonce        *sync.Once
}

func (em *expirationManager) stop() {
	em.killonce.Do(func() {
		close(em.kill)
	})
}

func (em *expirationManager) start(ctx context.Context) {
	ticker := time.NewTicker(em.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-em.kill:
			return
		case <-ticker.C:
			if !em.initialScanDone {
				em.initialScan(ctx)
				em.initialScanDone = true
			}

			em.checkExpiredEvents(ctx)
		}
	}
}

func (em *expirationManager) initialScan(ctx context.Context) {
	em.mu.Lock()
	defer em.mu.Unlock()

	// query all events
	ctx = context.WithValue(ctx, internalCallKey, struct{}{})
	for evt := range em.queryStored(ctx, nostr.Filter{}) {
		if expiresAt := nip40.GetExpiration(evt.Tags); expiresAt != -1 {
			heap.Push(&em.events, expiringEvent{
				id:        evt.ID,
				expiresAt: expiresAt,
			})
		}
	}

	heap.Init(&em.events)
}

func (em *expirationManager) checkExpiredEvents(ctx context.Context) {
	em.mu.Lock()
	defer em.mu.Unlock()

	now := nostr.Now()

	// keep deleting events from the heap as long as they're expired
	for em.events.Len() > 0 {
		next := em.events[0]
		if now < next.expiresAt {
			break
		}

		heap.Pop(&em.events)

		ctx := context.WithValue(ctx, internalCallKey, struct{}{})
		em.deleteEvent(ctx, next.id)
	}
}

func (em *expirationManager) trackEvent(evt nostr.Event) {
	if expiresAt := nip40.GetExpiration(evt.Tags); expiresAt != -1 {
		em.mu.Lock()
		heap.Push(&em.events, expiringEvent{
			id:        evt.ID,
			expiresAt: expiresAt,
		})
		em.mu.Unlock()
	}
}

func (em *expirationManager) removeEvent(id nostr.ID) {
	em.mu.Lock()
	defer em.mu.Unlock()

	// Find and remove the event from the heap
	for i := 0; i < len(em.events); i++ {
		if em.events[i].id == id {
			heap.Remove(&em.events, i)
			break
		}
	}
}

func (rl *Relay) StartExpirationManager(
	queryStored func(ctx context.Context, filter nostr.Filter) iter.Seq[nostr.Event],
	deleteEvent func(ctx context.Context, id nostr.ID) error,
) {
	rl.expirationManager = &expirationManager{
		events: make(expiringEventHeap, 0),

		queryStored: queryStored,
		deleteEvent: deleteEvent,

		interval: time.Hour,
		kill:     make(chan struct{}),
		killonce: &sync.Once{},
	}

	go rl.expirationManager.start(rl.ctx)
	rl.Info.AddSupportedNIP(40)
}

func (rl *Relay) DisableExpirationManager() {
	rl.expirationManager.stop()
	rl.expirationManager = nil

	idx := slices.Index(rl.Info.SupportedNIPs, 40)
	if idx != -1 {
		rl.Info.SupportedNIPs[idx] = rl.Info.SupportedNIPs[len(rl.Info.SupportedNIPs)-1]
		rl.Info.SupportedNIPs = rl.Info.SupportedNIPs[0 : len(rl.Info.SupportedNIPs)-1]
	}
}
