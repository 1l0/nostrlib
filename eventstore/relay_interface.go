package eventstore

import (
	"context"
	"fmt"

	"fiatjaf.com/nostr"
)

type RelayWrapper struct {
	Store
}

func (w RelayWrapper) Publish(ctx context.Context, evt nostr.Event) error {
	if nostr.IsEphemeralKind(evt.Kind) {
		// do not store ephemeral events
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if nostr.IsRegularKind(evt.Kind) {
		// regular events are just saved directly
		if err := w.SaveEvent(evt); err != nil && err != ErrDupEvent {
			return fmt.Errorf("failed to save: %w", err)
		}
		return nil
	}

	// others are replaced
	w.Store.ReplaceEvent(evt)

	return nil
}
