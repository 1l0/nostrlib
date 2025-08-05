package bolt

import (
	"os"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/eventstore"
	"go.etcd.io/bbolt"
)

var (
	settingsStore   = []byte("settingsStore")
	rawEventStore   = []byte("rawEventStore")
	indexCreatedAt  = []byte("indexCreatedAt")
	indexKind       = []byte("indexKind")
	indexPubkey     = []byte("indexPubkey")
	indexPubkeyKind = []byte("indexPubkeyKind")
	indexTag        = []byte("indexTag")
	indexTag32      = []byte("indexTag32")
	indexTagAddr    = []byte("indexTagAddr")
	hllCache        = []byte("hllCache")
)

var _ eventstore.Store = (*BoltBackend)(nil)

type BoltBackend struct {
	Path    string
	MapSize int64
	DB      *bbolt.DB

	EnableHLLCacheFor func(kind nostr.Kind) (useCache bool, skipSavingActualEvent bool)
}

func (b *BoltBackend) Init() error {
	// create directory if it doesn't exist and open it
	if err := os.MkdirAll(b.Path, 0755); err != nil {
		return err
	}

	return b.initialize()
}

func (b *BoltBackend) Close() {
	b.DB.Close()
}

func (b *BoltBackend) initialize() error {
	db, err := bbolt.Open(b.Path, 0600, nil)
	if err != nil {
		return err
	}

	db.Update(func(txn *bbolt.Tx) error {
		if _, err := txn.CreateBucketIfNotExists(settingsStore); err != nil {
			return err
		}
		if _, err := txn.CreateBucketIfNotExists(rawEventStore); err != nil {
			return err
		}
		if _, err := txn.CreateBucketIfNotExists(indexCreatedAt); err != nil {
			return err
		}
		if _, err := txn.CreateBucketIfNotExists(indexKind); err != nil {
			return err
		}
		if _, err := txn.CreateBucketIfNotExists(indexPubkey); err != nil {
			return err
		}
		if _, err := txn.CreateBucketIfNotExists(indexPubkeyKind); err != nil {
			return err
		}
		if _, err := txn.CreateBucketIfNotExists(indexTag); err != nil {
			return err
		}
		if _, err := txn.CreateBucketIfNotExists(indexTag32); err != nil {
			return err
		}
		if _, err := txn.CreateBucketIfNotExists(indexTagAddr); err != nil {
			return err
		}
		if _, err := txn.CreateBucketIfNotExists(hllCache); err != nil {
			return err
		}
		return nil
	})

	return b.migrate()
}
