//go:build !tinygo

package nip59

import (
	"fiatjaf.com/nostr"
	"github.com/mailru/easyjson"
)

func unmarshalEvent(data []byte, evt *nostr.Event) error {
	return easyjson.Unmarshal(data, evt)
}
