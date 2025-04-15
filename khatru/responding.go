package khatru

import (
	"context"
	"errors"
	"sync"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip45/hyperloglog"
)

func (rl *Relay) handleRequest(ctx context.Context, id string, eose *sync.WaitGroup, ws *WebSocket, filter nostr.Filter) error {
	defer eose.Done()

	if filter.LimitZero {
		// don't do any queries, just subscribe to future events
		return nil
	}

	// then check if we'll reject this filter (we apply this after overwriting
	// because we may, for example, remove some things from the incoming filters
	// that we know we don't support, and then if the end result is an empty
	// filter we can just reject it)
	if rl.RejectFilter != nil {
		if reject, msg := rl.RejectFilter(ctx, filter); reject {
			return errors.New(nostr.NormalizeOKMessage(msg, "blocked"))
		}
	}

	// run the function to query events
	if rl.QueryEvents != nil {
		ch, err := rl.QueryEvents(ctx, filter)
		if err != nil {
			ws.WriteJSON(nostr.NoticeEnvelope(err.Error()))
			eose.Done()
		} else if ch == nil {
			eose.Done()
		}

		go func(ch chan *nostr.Event) {
			for event := range ch {
				ws.WriteJSON(nostr.EventEnvelope{SubscriptionID: &id, Event: *event})
			}
			eose.Done()
		}(ch)
	}

	return nil
}

func (rl *Relay) handleCountRequest(ctx context.Context, ws *WebSocket, filter nostr.Filter) int64 {
	// check if we'll reject this filter
	if rl.RejectCountFilter != nil {
		if rejecting, msg := rl.RejectCountFilter(ctx, filter); rejecting {
			ws.WriteJSON(nostr.NoticeEnvelope(msg))
			return 0
		}
	}

	// run the functions to count (generally it will be just one)
	var subtotal int64 = 0
	if rl.CountEvents != nil {
		res, err := rl.CountEvents(ctx, filter)
		if err != nil {
			ws.WriteJSON(nostr.NoticeEnvelope(err.Error()))
		}
		subtotal += res
	}

	return subtotal
}

func (rl *Relay) handleCountRequestWithHLL(
	ctx context.Context,
	ws *WebSocket,
	filter nostr.Filter,
	offset int,
) (int64, *hyperloglog.HyperLogLog) {
	// check if we'll reject this filter
	if rl.RejectCountFilter != nil {
		if rejecting, msg := rl.RejectCountFilter(ctx, filter); rejecting {
			ws.WriteJSON(nostr.NoticeEnvelope(msg))
			return 0, nil
		}
	}

	// run the functions to count (generally it will be just one)
	var subtotal int64 = 0
	var hll *hyperloglog.HyperLogLog
	if rl.CountEventsHLL != nil {
		res, fhll, err := rl.CountEventsHLL(ctx, filter, offset)
		if err != nil {
			ws.WriteJSON(nostr.NoticeEnvelope(err.Error()))
		}
		subtotal += res
		if fhll != nil {
			if hll == nil {
				hll = fhll
			} else {
				hll.Merge(fhll)
			}
		}
	}

	return subtotal, hll
}
