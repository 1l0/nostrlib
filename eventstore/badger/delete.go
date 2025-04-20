package badger

import (
	"fmt"
	"log"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/eventstore/codec/betterbinary"
	"github.com/dgraph-io/badger/v4"
)

var serialDelete uint32 = 0

func (b *BadgerBackend) DeleteEvent(id nostr.ID) error {
	fmt.Println("...", id)

	deletionHappened := false

	err := b.Update(func(txn *badger.Txn) error {
		var err error
		deletionHappened, err = b.delete(txn, id)
		return err
	})
	if err != nil {
		return err
	}

	// after deleting, run garbage collector (sometimes)
	if deletionHappened {
		serialDelete = (serialDelete + 1) % 256
		if serialDelete == 0 {
			if err := b.RunValueLogGC(0.8); err != nil && err != badger.ErrNoRewrite {
				log.Println("badger gc errored:" + err.Error())
			}
		}
	}

	return nil
}

func (b *BadgerBackend) delete(txn *badger.Txn, id nostr.ID) (bool, error) {
	idx := make([]byte, 1, 5)
	idx[0] = rawEventStorePrefix

	// query event by id to get its idx
	prefix := make([]byte, 1+8)
	prefix[0] = indexIdPrefix
	copy(prefix[1:], id[0:8])
	opts := badger.IteratorOptions{
		PrefetchValues: false,
	}

	// also grab the actual event so we can calculate its indexes
	var evt nostr.Event

	it := txn.NewIterator(opts)
	defer it.Close()
	it.Seek(prefix)
	if it.ValidForPrefix(prefix) {
		idx = append(idx, it.Item().Key()[1+8:1+8+4]...)
		item, err := txn.Get(idx)
		if err == badger.ErrKeyNotFound {
			// this event doesn't exist or is already deleted
			return false, nil
		} else if err != nil {
			return false, fmt.Errorf("failed to fetch event %x to delete: %w", id[:], err)
		} else {
			if err := item.Value(func(bin []byte) error {
				return betterbinary.Unmarshal(bin, &evt)
			}); err != nil {
				return false, fmt.Errorf("failed to unmarshal event %x to delete: %w", id[:], err)
			}
		}
	}

	// calculate all index keys we have for this event and delete them
	for k := range b.getIndexKeysForEvent(evt, idx[1:]) {
		if err := txn.Delete(k); err != nil {
			return false, err
		}
	}

	// delete the raw event
	return true, txn.Delete(idx)
}
