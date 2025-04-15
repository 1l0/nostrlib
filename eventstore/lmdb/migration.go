package lmdb

import (
	"encoding/binary"
	"fmt"

	"github.com/PowerDNS/lmdb-go/lmdb"
)

const (
	DB_VERSION byte = 'v'
)

func (b *LMDBBackend) runMigrations() error {
	return b.lmdbEnv.Update(func(txn *lmdb.Txn) error {
		var version uint16
		v, err := txn.Get(b.settingsStore, []byte{DB_VERSION})
		if err != nil {
			if lmdb.IsNotFound(err) {
				version = 0
			} else if v == nil {
				return fmt.Errorf("failed to read database version: %w", err)
			}
		} else {
			version = binary.BigEndian.Uint16(v)
		}

		// do the migrations in increasing steps (there is no rollback)
		//

		// this is when we reindex everything
		if version < 1 {
		}

		// bump version
		// if err := b.setVersion(txn, 1); err != nil {
		// 	return err
		// }

		return nil
	})
}

func (b *LMDBBackend) setVersion(txn *lmdb.Txn, version uint16) error {
	buf, err := txn.PutReserve(b.settingsStore, []byte{DB_VERSION}, 4, 0)
	binary.BigEndian.PutUint16(buf, version)
	return err
}
