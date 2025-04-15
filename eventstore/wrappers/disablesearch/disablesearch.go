package disablesearch

import (
	"context"

	"fiatjaf.com/nostr/eventstore"
	"fiatjaf.com/nostr"
)

type Wrapper struct {
	eventstore.Store
}

var _ eventstore.Store = (*Wrapper)(nil)

func (w Wrapper) QueryEvents(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
	if filter.Search != "" {
		return nil, nil
	}
	return w.Store.QueryEvents(ctx, filter)
}
