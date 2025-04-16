package nostr

import (
	"context"
)

type Publisher interface {
	Publish(context.Context, Event) error
}

type Querier interface {
	QueryEvents(context.Context, Filter) (chan Event, error)
}

type QuerierPublisher interface {
	Querier
	Publisher
}
