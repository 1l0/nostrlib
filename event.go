package nostr

import (
	"crypto/sha256"
)

// Event represents a Nostr event.
type Event struct {
	ID        ID
	PubKey    PubKey
	CreatedAt Timestamp
	Kind      Kind
	Tags      Tags
	Content   string
	Sig       [64]byte
}

// GetID serializes and returns the event ID as a string.
func (evt Event) GetID() ID {
	return sha256.Sum256(evt.Serialize())
}

// CheckID checks if the implied ID matches the given ID more efficiently.
func (evt Event) CheckID() bool {
	return evt.GetID() == evt.ID
}
