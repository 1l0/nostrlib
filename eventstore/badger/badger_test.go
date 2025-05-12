package badger

import (
	"os"
	"testing"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/eventstore"
	"github.com/stretchr/testify/require"
)

func TestBasicStoreAndQuery(t *testing.T) {
	// create a temporary directory for the test database
	dir, err := os.MkdirTemp("", "badger-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	// initialize the store
	db := &BadgerBackend{Path: dir}
	err = db.Init()
	require.NoError(t, err)
	defer db.Close()

	// create a test event
	evt := nostr.Event{
		Content:   "hello world",
		CreatedAt: 1000,
		Kind:      1,
		Tags:      nostr.Tags{},
	}
	err = evt.Sign(nostr.Generate())
	require.NoError(t, err)

	// save the event
	err = db.SaveEvent(evt)
	require.NoError(t, err)

	// try to save it again, should fail with ErrDupEvent
	err = db.SaveEvent(evt)
	require.Error(t, err)
	require.Equal(t, eventstore.ErrDupEvent, err)

	// query the event by its ID
	filter := nostr.Filter{
		IDs: []nostr.ID{evt.ID},
	}

	// collect results
	results := make([]nostr.Event, 0)
	for event := range db.QueryEvents(filter, 500) {
		results = append(results, event)
	}

	// verify we got exactly one event and it matches
	require.Len(t, results, 1)
	require.Equal(t, evt.ID, results[0].ID)
	require.Equal(t, evt.Content, results[0].Content)
	require.Equal(t, evt.CreatedAt, results[0].CreatedAt)
	require.Equal(t, evt.Kind, results[0].Kind)
	require.Equal(t, evt.PubKey, results[0].PubKey)
}
