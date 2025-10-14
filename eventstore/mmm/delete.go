package mmm

import (
	"encoding/binary"
	"fmt"
	"slices"

	"fiatjaf.com/nostr"
	"github.com/PowerDNS/lmdb-go/lmdb"
)

func (il *IndexingLayer) DeleteEvent(id nostr.ID) error {
	il.mmmm.writeMutex.Lock()
	defer il.mmmm.writeMutex.Unlock()

	return il.mmmm.lmdbEnv.Update(func(mmmtxn *lmdb.Txn) error {
		return il.lmdbEnv.Update(func(iltxn *lmdb.Txn) error {
			return il.delete(mmmtxn, iltxn, id)
		})
	})
}

func (il *IndexingLayer) delete(mmmtxn *lmdb.Txn, iltxn *lmdb.Txn, id nostr.ID) error {
	zeroRefs := false
	b := il.mmmm

	b.Logger.Debug().Str("layer", il.name).Uint16("il", il.id).Msg("deleting")

	// first in the mmmm txn we check if we have the event still
	val, err := mmmtxn.Get(b.indexId, id[0:8])
	if err != nil {
		if lmdb.IsNotFound(err) {
			// we already do not have this anywhere
			return nil
		}
		return fmt.Errorf("failed to check if we have the event %x: %w", id, err)
	}

	// we have this, but do we have it in the current layer?
	// val is [posb][il_idx][il_idx...]
	pos := positionFromBytes(val[0:12])

	// check references
	currentLayer := binary.BigEndian.AppendUint16(nil, il.id)
	for i := 12; i < len(val); i += 2 {
		if slices.Equal(val[i:i+2], currentLayer) {
			// we will remove the current layer if it's found
			nextval := make([]byte, len(val)-2)
			copy(nextval, val[0:i])
			copy(nextval[i:], val[i+2:])

			if err := mmmtxn.Put(b.indexId, id[0:8], nextval, 0); err != nil {
				return fmt.Errorf("failed to update references for %x: %w", id[:], err)
			}

			// if there are no more layers we will delete everything later
			zeroRefs = len(nextval) == 12

			break
		}
	}

	// load the event so we can compute the indexes
	var evt nostr.Event
	if err := il.mmmm.loadEvent(pos, &evt); err != nil {
		return fmt.Errorf("failed to load event %x when deleting: %w", id[:], err)
	}

	if err := il.deleteIndexes(iltxn, evt, val[0:12]); err != nil {
		return fmt.Errorf("failed to delete indexes for %s=>%v: %w", evt.ID, val[0:12], err)
	}

	// if there are no more refs we delete the event from the id index and mmap
	if zeroRefs {
		if err := b.purge(mmmtxn, id[0:8], pos); err != nil {
			panic(err)
		}
	}

	return nil
}

func (il *IndexingLayer) deleteIndexes(iltxn *lmdb.Txn, event nostr.Event, posbytes []byte) error {
	// calculate all index keys we have for this event and delete them
	for k := range il.getIndexKeysForEvent(event) {
		if err := iltxn.Del(k.dbi, k.key, posbytes); err != nil && !lmdb.IsNotFound(err) {
			return fmt.Errorf("index entry %v/%x deletion failed: %w", k.dbi, k.key, err)
		}
	}

	return nil
}
