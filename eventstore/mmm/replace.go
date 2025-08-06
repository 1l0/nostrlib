package mmm

import (
	"fmt"
	"iter"
	"runtime"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/eventstore/internal"
	"github.com/PowerDNS/lmdb-go/lmdb"
)

func (il *IndexingLayer) ReplaceEvent(evt nostr.Event) error {
	il.mmmm.writeMutex.Lock()
	defer il.mmmm.writeMutex.Unlock()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	filter := nostr.Filter{Kinds: []nostr.Kind{evt.Kind}, Authors: []nostr.PubKey{evt.PubKey}}
	if evt.Kind.IsAddressable() {
		// when addressable, add the "d" tag to the filter
		filter.Tags = nostr.TagMap{"d": []string{evt.Tags.GetD()}}
	}

	return il.mmmm.lmdbEnv.Update(func(mmmtxn *lmdb.Txn) error {
		mmmtxn.RawRead = true

		return il.lmdbEnv.Update(func(iltxn *lmdb.Txn) error {
			// now we fetch the past events, whatever they are, delete them and then save the new
			var err error
			var results iter.Seq[nostr.Event] = func(yield func(nostr.Event) bool) {
				err = il.query(iltxn, filter, 10 /* in theory limit could be just 1 and this should work */, yield)
			}
			if err != nil {
				return fmt.Errorf("failed to query past events with %s: %w", filter, err)
			}

			shouldStore := true
			for previous := range results {
				if internal.IsOlder(previous, evt) {
					if err := il.delete(mmmtxn, iltxn, previous.ID); err != nil {
						return fmt.Errorf("failed to delete event %s for replacing: %w", previous.ID, err)
					}
				} else {
					// there is a newer event already stored, so we won't store this
					shouldStore = false
				}
			}
			if shouldStore {
				_, err := il.mmmm.storeOn(mmmtxn, []*IndexingLayer{il}, []*lmdb.Txn{iltxn}, evt)
				return err
			}

			return nil
		})
	})
}
