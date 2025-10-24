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

var ZeroID = ID{}

// ID represents an event id
type ID [32]byte

func (id ID) String() string { return "id::" + id.Hex() }
func (id ID) Hex() string    { return hex.EncodeToString(id[:]) }

func (id ID) MarshalJSON() ([]byte, error) {
	res := make([]byte, 66)
	hex.Encode(res[1:], id[:])
	res[0] = '"'
	res[65] = '"'
	return res, nil
}

func (id *ID) UnmarshalJSON(buf []byte) error {
	if len(buf) != 66 {
		return fmt.Errorf("must be a hex string of 64 characters")
	}
	_, err := hex.Decode(id[:], buf[1:65])
	return err
}

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
	if _, err := hex.Decode(id[:], unsafe.Slice(unsafe.StringData(idh), 64)); err != nil {
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
