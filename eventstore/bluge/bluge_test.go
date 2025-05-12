package bluge

import (
	"os"
	"testing"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/eventstore/badger"
	"github.com/stretchr/testify/assert"
)

func TestBlugeFlow(t *testing.T) {
	os.RemoveAll("/tmp/blugetest-badger")
	os.RemoveAll("/tmp/blugetest-bluge")

	bb := &badger.BadgerBackend{Path: "/tmp/blugetest-badger"}
	bb.Init()
	defer bb.Close()

	bl := BlugeBackend{
		Path:          "/tmp/blugetest-bluge",
		RawEventStore: bb,
	}
	bl.Init()
	defer bl.Close()

	willDelete := make([]nostr.Event, 0, 3)

	for i, content := range []string{
		"good morning mr paper maker",
		"good night",
		"I'll see you again in the paper house",
		"tonight we dine in my house",
		"the paper in this house if very good, mr",
	} {
		evt := nostr.Event{Content: content, Tags: nostr.Tags{}}
		evt.Sign(nostr.MustSecretKeyFromHex("0000000000000000000000000000000000000000000000000000000000000001"))

		bb.SaveEvent(evt)
		bl.SaveEvent(evt)

		if i%2 == 0 {
			willDelete = append(willDelete, evt)
		}
	}

	{
		n := 0
		for range bl.QueryEvents(nostr.Filter{Search: "good"}, 400) {
			n++
		}
		assert.Equal(t, 3, n)
	}

	for _, evt := range willDelete {
		bl.DeleteEvent(evt.ID)
	}

	{
		n := 0
		for res := range bl.QueryEvents(nostr.Filter{Search: "good"}, 400) {
			n++
			assert.Equal(t, res.Content, "good night")
			assert.Equal(t,
				nostr.MustPubKeyFromHex("79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"),
				res.PubKey,
			)
		}
		assert.Equal(t, 1, n)
	}
}
