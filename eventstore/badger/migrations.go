package badger

import (
	"encoding/binary"

	"github.com/dgraph-io/badger/v4"
)

func (b *BadgerBackend) runMigrations() error {
	return b.Update(func(txn *badger.Txn) error {
		var version uint16

		item, err := txn.Get([]byte{dbVersionKey})
		if err == badger.ErrKeyNotFound {
			version = 0
		} else if err != nil {
			return err
		} else {
			item.Value(func(val []byte) error {
				version = binary.BigEndian.Uint16(val)
				return nil
			})
		}

		// do the migrations in increasing steps (there is no rollback)
		//

		if version < 1 {
			// ...
		}

		// b.bumpVersion(txn, 1)

		return nil
	})
}

func (b *BadgerBackend) bumpVersion(txn *badger.Txn, version uint16) error {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, version)
	return txn.Set([]byte{dbVersionKey}, buf)
}
