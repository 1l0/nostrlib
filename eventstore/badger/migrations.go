package badger

import (
	"encoding/binary"
	"fmt"
	"log"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/eventstore/codec/betterbinary"
	"github.com/dgraph-io/badger/v4"
)

func (b *BadgerBackend) migrate() error {
	return b.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("version"))
		if err != nil && err != badger.ErrKeyNotFound {
			return fmt.Errorf("failed to get db version: %w", err)
		}

		var version uint16 = 1
		if err == nil {
			err = item.Value(func(val []byte) error {
				version = binary.BigEndian.Uint16(val)
				return nil
			})
			if err != nil {
				return fmt.Errorf("failed to read db version: %w", err)
			}
		}

		const target = 2

		// do the migrations in increasing steps (there is no rollback)
		if version < target {
			log.Printf("[badger] migration %d: delete all indexes and recreate them\n", target)

			// delete all index entries
			prefixes := []byte{
				indexIdPrefix,
				indexCreatedAtPrefix,
				indexKindPrefix,
				indexPubkeyPrefix,
				indexPubkeyKindPrefix,
				indexTagPrefix,
				indexTag32Prefix,
				indexTagAddrPrefix,
			}

			for _, prefix := range prefixes {
				it := txn.NewIterator(badger.IteratorOptions{
					PrefetchValues: false,
					Prefix:         []byte{prefix},
				})
				defer it.Close()

				var keysToDelete [][]byte
				for it.Seek([]byte{prefix}); it.ValidForPrefix([]byte{prefix}); it.Next() {
					key := it.Item().Key()
					keyCopy := make([]byte, len(key))
					copy(keyCopy, key)
					keysToDelete = append(keysToDelete, keyCopy)
				}

				for _, key := range keysToDelete {
					if err := txn.Delete(key); err != nil {
						return fmt.Errorf("failed to delete index key %x: %w", key, err)
					}
				}
			}

			// iterate through all events and recreate indexes
			it := txn.NewIterator(badger.IteratorOptions{
				PrefetchValues: true,
				Prefix:         []byte{rawEventStorePrefix},
			})
			defer it.Close()

			for it.Seek([]byte{rawEventStorePrefix}); it.ValidForPrefix([]byte{rawEventStorePrefix}); it.Next() {
				item := it.Item()
				idx := item.Key()

				err := item.Value(func(val []byte) error {
					evt := nostr.Event{}
					if err := betterbinary.Unmarshal(val, &evt); err != nil {
						return fmt.Errorf("error decoding event %x on migration %d: %w", idx, target, err)
					}

					for key := range b.getIndexKeysForEvent(evt, idx[1:]) {
						if err := txn.Set(key, nil); err != nil {
							return fmt.Errorf("failed to save index for event %s on migration %d: %w", evt.ID, target, err)
						}
					}

					return nil
				})
				if err != nil {
					return err
				}
			}

			// bump version
			if err := b.bumpVersion(txn, target); err != nil {
				return err
			}
		}

		return nil
	})
}

func (b *BadgerBackend) bumpVersion(txn *badger.Txn, v uint16) error {
	var newVersion [2]byte
	binary.BigEndian.PutUint16(newVersion[:], v)
	return txn.Set([]byte("version"), newVersion[:])
}
