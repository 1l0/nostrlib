//go:build !tinygo

package nip46

import (
	"unsafe"

	"fiatjaf.com/nostr"
	"github.com/mailru/easyjson"
	"github.com/puzpuzpuz/xsync/v3"
)

type listenersMap = xsync.MapOf[string, chan Response]

func newListenersMap() *listenersMap {
	return xsync.NewMapOf[string, chan Response]()
}

func unmarshalEvent(data string, evt *nostr.Event) error {
	return easyjson.Unmarshal(unsafe.Slice(unsafe.StringData(data), len(data)), evt)
}

func marshalEvent(evt *nostr.Event) ([]byte, error) {
	return easyjson.Marshal(evt)
}
