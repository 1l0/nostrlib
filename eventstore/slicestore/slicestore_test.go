package slicestore

import (
	"testing"

	"fiatjaf.com/nostr"
)

func TestBasicStuff(t *testing.T) {
	ss := &SliceStore{}
	ss.Init()
	defer ss.Close()

	for i := 0; i < 20; i++ {
		v := i
		kind := 11
		if i%2 == 0 {
			v = i + 10000
		}
		if i%3 == 0 {
			kind = 12
		}
		ss.SaveEvent(nostr.Event{CreatedAt: nostr.Timestamp(v), Kind: uint16(kind)})
	}

	list := make([]nostr.Event, 0, 20)
	for event := range ss.QueryEvents(nostr.Filter{}) {
		list = append(list, event)
	}

	if len(list) != 20 {
		t.Fatalf("failed to load 20 events")
	}
	if list[0].CreatedAt != 10018 || list[1].CreatedAt != 10016 || list[18].CreatedAt != 3 || list[19].CreatedAt != 1 {
		t.Fatalf("order is incorrect")
	}

	until := nostr.Timestamp(9999)
	list = make([]nostr.Event, 0, 7)
	for event := range ss.QueryEvents(nostr.Filter{Limit: 15, Until: &until, Kinds: []uint16{11}}) {
		list = append(list, event)
	}
	if len(list) != 7 {
		t.Fatalf("should have gotten 7, not %d", len(list))
	}

	since := nostr.Timestamp(10009)
	list = make([]nostr.Event, 0, 5)
	for event := range ss.QueryEvents(nostr.Filter{Since: &since}) {
		list = append(list, event)
	}
	if len(list) != 5 {
		t.Fatalf("should have gotten 5, not %d", len(list))
	}
}
