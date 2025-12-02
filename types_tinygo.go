//go:build tinygo

package nostr

import (
	"encoding/hex"
	stdjson "encoding/json"
	"fmt"
)

// RelayEvent represents an event received from a specific relay.
type RelayEvent struct {
	Event
	Relay *Relay
}

var ZeroID = ID{}

// ID represents an event id
type ID [32]byte

var (
	_ stdjson.Marshaler   = ID{}
	_ stdjson.Unmarshaler = (*ID)(nil)
)

func (id ID) String() string { return "id::" + id.Hex() }
func (id ID) Hex() string    { return hex.EncodeToString(id[:]) }

func (id ID) MarshalJSON() ([]byte, error) {
	return stdjson.Marshal(id.Hex())
}

func (id *ID) UnmarshalJSON(buf []byte) error {
	var s string
	if err := stdjson.Unmarshal(buf, &s); err != nil {
		return err
	}
	if len(s) != 64 {
		return fmt.Errorf("must be a hex string of 64 characters")
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	copy(id[:], b)
	return nil
}

func IDFromHex(idh string) (ID, error) {
	id := ID{}

	if len(idh) != 64 {
		return id, fmt.Errorf("pubkey should be 64-char hex, got '%s'", idh)
	}
	b, err := hex.DecodeString(idh)
	if err != nil {
		return id, fmt.Errorf("'%s' is not valid hex: %w", idh, err)
	}
	copy(id[:], b)

	return id, nil
}

func MustIDFromHex(idh string) ID {
	id, err := IDFromHex(idh)
	if err != nil {
		panic(err)
	}
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
