package bluge

import (
	"context"

	"fiatjaf.com/nostr"
)

func (b *BlugeBackend) DeleteEvent(ctx context.Context, evt *nostr.Event) error {
	return b.writer.Delete(eventIdentifier(evt.ID))
}
