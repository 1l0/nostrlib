package bluge

import (
	"fiatjaf.com/nostr"
)

func (b *BlugeBackend) DeleteEvent(id nostr.ID) error {
	return b.writer.Delete(eventIdentifier(id))
}
