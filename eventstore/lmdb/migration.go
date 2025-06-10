package lmdb

import (
	"encoding/binary"
	"fmt"
	"log"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/eventstore/codec/betterbinary"
	"github.com/PowerDNS/lmdb-go/lmdb"
)

const (
	DB_VERSION byte = 'v'
)

func (b *LMDBBackend) runMigrations() error {
	return b.lmdbEnv.Update(func(txn *lmdb.Txn) error {
		val, err := txn.Get(b.settingsStore, []byte("version"))
		if err != nil && !lmdb.IsNotFound(err) {
			return fmt.Errorf("failed to get db version: %w", err)
		}

		var version uint16 = 0
		if err == nil {
			version = binary.BigEndian.Uint16(val)
		}

		// do the migrations in increasing steps (there is no rollback)
		//

		// this is when we reindex everything
		if version < 9 {
			log.Println("[lmdb] migration 9: reindex everything")

			if err := txn.Drop(b.indexId, false); err != nil {
				return err
			}
			if err := txn.Drop(b.indexKind, false); err != nil {
				return err
			}
			if err := txn.Drop(b.indexPubkey, false); err != nil {
				return err
			}
			if err := txn.Drop(b.indexPubkeyKind, false); err != nil {
				return err
			}
			if err := txn.Drop(b.indexTag, false); err != nil {
				return err
			}
			if err := txn.Drop(b.indexTag32, false); err != nil {
				return err
			}
			if err := txn.Drop(b.indexTagAddr, false); err != nil {
				return err
			}
			if err := txn.Drop(b.indexPTagKind, false); err != nil {
				return err
			}

			cursor, err := txn.OpenCursor(b.rawEventStore)
			if err != nil {
				return fmt.Errorf("failed to open cursor in migration 9: %w", err)
			}
			defer cursor.Close()

			var idx, val []byte
			var evt nostr.Event

			for {
				idx, val, err = cursor.Get(nil, nil, lmdb.Next)
				if lmdb.IsNotFound(err) {
					break
				}
				if err != nil {
					return fmt.Errorf("failed to get next in migration 9: %w", err)
				}

				if err := betterbinary.Unmarshal(val, &evt); err != nil {
					log.Printf("failed to unmarshal event %x, skipping: %s", idx, err)
					continue
				}

				for key := range b.getIndexKeysForEvent(evt) {
					if err := txn.Put(key.dbi, key.key, idx, 0); err != nil {
						return fmt.Errorf("failed to save index %s for event %s (%v) on migration 9: %w",
							b.keyName(key), evt.ID, idx, err)
					}
				}
			}

			// bump version
			if err := b.setVersion(txn, 9); err != nil {
				return err
			}
		}

		return nil
	})
}

func (b *LMDBBackend) setVersion(txn *lmdb.Txn, v uint16) error {
	var newVersion [2]byte
	binary.BigEndian.PutUint16(newVersion[:], v)
	return txn.Put(b.settingsStore, []byte("version"), newVersion[:], 0)
}
