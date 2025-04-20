package wrappers

import (
	"context"
	"fmt"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/eventstore"
)

var _ nostr.Publisher = StorePublisher{}

type StorePublisher struct {
	eventstore.Store
}

func (w StorePublisher) Publish(ctx context.Context, evt nostr.Event) error {
	if evt.Kind.IsEphemeral() {
		// do not store ephemeral events
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if evt.Kind.IsRegular() {
		// regular events are just saved directly
		if err := w.SaveEvent(evt); err != nil && err != eventstore.ErrDupEvent {
			return fmt.Errorf("failed to save: %w", err)
		}
		return nil
	}

	// others are replaced
	w.Store.ReplaceEvent(evt)

	return nil
}
