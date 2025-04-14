package sdk

import (
	"slices"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/sdk/kvstore"
)

const eventRelayPrefix = byte('r')

// makeEventRelayKey creates a key for storing event relay information.
// It uses the first 8 bytes of the event ID to create a compact key.
func makeEventRelayKey(id nostr.ID) []byte {
	// format: 'r' + first 8 bytes of event ID
	key := make([]byte, 9)
	key[0] = eventRelayPrefix
	copy(key[1:], id[:8])
	return key
}

// encodeRelayList serializes a list of relay URLs into a compact binary format.
// Each relay URL is prefixed with its length as a single byte.
func encodeRelayList(relays []string) []byte {
	totalSize := 0
	for _, relay := range relays {
		if len(relay) > 256 {
			continue
		}
		totalSize += 1 + len(relay) // 1 byte for length prefix
	}

	buf := make([]byte, totalSize)
	offset := 0

	for _, relay := range relays {
		if len(relay) > 256 {
			continue
		}
		buf[offset] = uint8(len(relay))
		offset += 1
		copy(buf[offset:], relay)
		offset += len(relay)
	}

	return buf
}

// decodeRelayList deserializes a binary-encoded list of relay URLs.
// It expects each relay URL to be prefixed with its length as a single byte.
func decodeRelayList(data []byte) []string {
	relays := make([]string, 0, 6)
	offset := 0

	for offset < len(data) {
		if offset+1 > len(data) {
			return nil // malformed
		}

		length := int(data[offset])
		offset += 1

		if offset+length > len(data) {
			return nil // malformed
		}

		relay := string(data[offset : offset+length])
		relays = append(relays, relay)
		offset += length
	}

	return relays
}

// trackEventRelay records that an event was seen on a particular relay.
// If onlyIfItExists is true, it will only update existing records and not create new ones.
func (sys *System) trackEventRelay(id nostr.ID, relay string, onlyIfItExists bool) {
	// get the key for this event
	key := makeEventRelayKey(id)

	// update the relay list atomically
	sys.KVStore.Update(key, func(data []byte) ([]byte, error) {
		var relays []string
		if data != nil {
			relays = decodeRelayList(data)

			// check if relay is already in list
			if slices.Contains(relays, relay) {
				return nil, kvstore.NoOp // no change needed
			}

			// append new relay
			relays = append(relays, relay)
			return encodeRelayList(relays), nil
		} else if onlyIfItExists {
			// when this flag exists and nothing was found we won't create anything
			return nil, kvstore.NoOp
		} else {
			// nothing exists, so create it
			return encodeRelayList([]string{relay}), nil
		}
	})
}

// GetEventRelays returns all known relay URLs an event is known to be available on.
// It is based on information kept on KVStore.
func (sys *System) GetEventRelays(id nostr.ID) ([]string, error) {
	// get the key for this event
	key := makeEventRelayKey(id)

	// get stored relay list
	data, err := sys.KVStore.Get(key)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	return decodeRelayList(data), nil
}
