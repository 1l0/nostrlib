//go:build tinygo

package nip59

import (
	"encoding/json"

	"fiatjaf.com/nostr"
)

func unmarshalEvent(data []byte, evt *nostr.Event) error {
	return json.Unmarshal(data, evt)
}
