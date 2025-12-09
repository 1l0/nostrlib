//go:build tinygo

package nip46

import (
	"sync"

	"fiatjaf.com/nostr"
)

type listenersMap struct {
	m sync.Map
}

func newListenersMap() *listenersMap {
	return &listenersMap{}
}

func (m *listenersMap) LoadAndDelete(key string) (chan Response, bool) {
	v, ok := m.m.LoadAndDelete(key)
	if !ok {
		return nil, false
	}
	return v.(chan Response), true
}

func (m *listenersMap) Store(key string, value chan Response) {
	m.m.Store(key, value)
}

func (m *listenersMap) Delete(key string) {
	m.m.Delete(key)
}

func unmarshalEvent(data string, evt *nostr.Event) error {
	return json.Unmarshal([]byte(data), evt)
}

func marshalEvent(evt *nostr.Event) ([]byte, error) {
	return json.Marshal(evt)
}
