package badger

import (
	"fmt"
	"math"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/eventstore"
	"fiatjaf.com/nostr/eventstore/codec/betterbinary"
	"github.com/dgraph-io/badger/v4"
)

func (b *BadgerBackend) SaveEvent(evt nostr.Event) error {
	// sanity checking
	if evt.CreatedAt > math.MaxUint32 || evt.Kind > math.MaxUint16 {
		return fmt.Errorf("event with values out of expected boundaries")
	}

	return b.Update(func(txn *badger.Txn) error {
		// query event by id to ensure we don't save duplicates
		prefix := make([]byte, 1+8)
		prefix[0] = indexIdPrefix
		copy(prefix[1:], evt.ID[0:8])
		it := txn.NewIterator(badger.IteratorOptions{})
		defer it.Close()
		it.Seek(prefix)
		if it.ValidForPrefix(prefix) {
			// event exists
			return eventstore.ErrDupEvent
		}

		return b.save(txn, evt)
	})
}

func (b *BadgerBackend) save(txn *badger.Txn, evt nostr.Event) error {
	// encode to binary
	buf := make([]byte, betterbinary.Measure(evt))
	if err := betterbinary.Marshal(evt, buf); err != nil {
		return err
	}

	idx := b.Serial()
	// raw event store
	if err := txn.Set(idx, buf); err != nil {
		return err
	}

	for k := range b.getIndexKeysForEvent(evt, idx[1:]) {
		if err := txn.Set(k, nil); err != nil {
			return err
		}
	}

	return nil
}
