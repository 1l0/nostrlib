package nostr

import (
	"encoding/hex"
	"fmt"
	"unsafe"
)

// RelayEvent represents an event received from a specific relay.
type RelayEvent struct {
	Event
	Relay *Relay
}

var ZeroID = [32]byte{}

// ID represents an event id
type ID [32]byte

func (id ID) String() string { return "id::" + id.Hex() }
func (id ID) Hex() string    { return hex.EncodeToString(id[:]) }

func IDFromHex(idh string) (ID, error) {
	id := ID{}

	if len(idh) != 64 {
		return id, fmt.Errorf("pubkey should be 64-char hex, got '%s'", idh)
	}
	if _, err := hex.Decode(id[:], unsafe.Slice(unsafe.StringData(idh), 64)); err != nil {
		return id, fmt.Errorf("'%s' is not valid hex: %w", idh, err)
	}

	return id, nil
}

func MustIDFromHex(idh string) ID {
	id := ID{}
	hex.Decode(id[:], unsafe.Slice(unsafe.StringData(idh), 64))
	return id
}

func ContainsID(haystack []ID, needle ID) bool {
	for _, cand := range haystack {
		if cand == needle {
			return true
		}
	}
	return false
}
