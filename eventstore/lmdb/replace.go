package lmdb

import (
	"fmt"
	"iter"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/eventstore/internal"
	"github.com/PowerDNS/lmdb-go/lmdb"
)

func (b *LMDBBackend) ReplaceEvent(evt nostr.Event) error {
	return b.lmdbEnv.Update(func(txn *lmdb.Txn) error {
		filter := nostr.Filter{Limit: 1, Kinds: []nostr.Kind{evt.Kind}, Authors: []nostr.PubKey{evt.PubKey}}
		if evt.Kind.IsAddressable() {
			// when addressable, add the "d" tag to the filter
			filter.Tags = nostr.TagMap{"d": []string{evt.Tags.GetD()}}
		}

		// now we fetch the past events, whatever they are, delete them and then save the new
		var yield_ func(nostr.Event) bool
		var results iter.Seq[nostr.Event] = func(yield func(nostr.Event) bool) {
			yield_ = yield
		}
		err := b.query(txn, filter, 10 /* in theory limit could be just 1 and this should work */, yield_)
		if err != nil {
			return fmt.Errorf("failed to query past events with %s: %w", filter, err)
		}

		shouldStore := true
		for previous := range results {
			if internal.IsOlder(previous, evt) {
				if err := b.delete(txn, previous.ID); err != nil {
					return fmt.Errorf("failed to delete event %s for replacing: %w", previous.ID, err)
				}
			} else {
				// there is a newer event already stored, so we won't store this
				shouldStore = false
			}
		}
		if shouldStore {
			return b.save(txn, evt)
		}

		return nil
	})
}
