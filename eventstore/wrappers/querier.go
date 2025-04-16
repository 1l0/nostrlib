package wrappers

import (
	"context"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/eventstore"
)

var _ nostr.Querier = StoreQuerier{}

type StoreQuerier struct {
	eventstore.Store
}

func (w StoreQuerier) QueryEvents(ctx context.Context, filter nostr.Filter) (chan nostr.Event, error) {
	ch := make(chan nostr.Event)

	go func() {
		for evt := range w.Store.QueryEvents(filter) {
			ch <- evt
		}
	}()

	return ch, nil
}
