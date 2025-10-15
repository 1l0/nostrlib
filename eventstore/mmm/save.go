package mmm

import (
	"encoding/binary"
	"fmt"
	"os"
	"runtime"
	"slices"
	"syscall"
	"unsafe"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/eventstore/codec/betterbinary"
	"github.com/PowerDNS/lmdb-go/lmdb"
)

func (il *IndexingLayer) SaveEvent(evt nostr.Event) error {
	il.mmmm.writeMutex.Lock()
	defer il.mmmm.writeMutex.Unlock()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// do this just so it's cleaner, we're already locking the thread and the mutex anyway
	mmmtxn, err := il.mmmm.lmdbEnv.BeginTxn(nil, 0)
	if err != nil {
		return fmt.Errorf("failed to begin global transaction: %w", err)
	}
	mmmtxn.RawRead = true

	iltxn, err := il.lmdbEnv.BeginTxn(nil, 0)
	if err != nil {
		mmmtxn.Abort()
		return fmt.Errorf("failed to start txn on %s: %w", il.name, err)
	}

	if _, err := il.mmmm.storeOn(mmmtxn, []*IndexingLayer{il}, []*lmdb.Txn{iltxn}, evt); err != nil {
		mmmtxn.Abort()
		if iltxn != nil {
			iltxn.Abort()
		}
		return err
	}

	if err := mmmtxn.Commit(); err != nil {
		return err
	}

	if err := iltxn.Commit(); err != nil {
		return err
	}

	return nil
}

func (b *MultiMmapManager) storeOn(
	mmmtxn *lmdb.Txn,
	ils []*IndexingLayer,
	iltxns []*lmdb.Txn,
	evt nostr.Event,
) (stored bool, err error) {
	// check if we already have this id
	val, err := mmmtxn.Get(b.indexId, evt.ID[0:8])
	if err == nil {
		// we found the event, now check if it is already indexed by the layers that want to store it
		for i := len(ils) - 1; i >= 0; i-- {
			for s := 12; s < len(val); s += 2 {
				ilid := binary.BigEndian.Uint16(val[s : s+2])
				if ils[i].id == ilid {
					// swap delete this il, but keep the deleted ones at the end
					// (so the caller can successfully finalize the transactions)
					ils[i], ils[len(ils)-1] = ils[len(ils)-1], ils[i]
					ils = ils[0 : len(ils)-1]
					iltxns[i], iltxns[len(iltxns)-1] = iltxns[len(iltxns)-1], iltxns[i]
					iltxns = iltxns[0 : len(iltxns)-1]
					break
				}
			}
		}
	} else if !lmdb.IsNotFound(err) {
		// now if we got an error from lmdb we will only proceed if we get a NotFound -- for anything else we will error
		return false, fmt.Errorf("error checking existence: %w", err)
	}

	// if all ils already have this event indexed (or no il was given) we can end here
	if len(ils) == 0 {
		return false, nil
	}

	// get event binary size
	pos := position{
		size: uint32(betterbinary.Measure(evt)),
	}
	if pos.size >= 1<<16 {
		return false, fmt.Errorf("event too large to store, max %d, got %d", 1<<16, pos.size)
	}

	// find a suitable place for this to be stored in
	appendToMmap := true
	for f, fr := range b.freeRanges {
		if fr.size >= pos.size {
			// found the smallest possible place that can fit this event
			appendToMmap = false
			pos.start = fr.start

			// modify the free ranges we're keeping track of
			if pos.size == fr.size {
				// if we've used it entirely just delete it
				b.freeRanges = slices.Delete(b.freeRanges, f, f+1)
			} else {
				// otherwise modify it in place
				b.freeRanges[f] = position{
					start: fr.start + uint64(pos.size),
					size:  fr.size - pos.size,
				}
			}

			break
		}
	}

	if appendToMmap {
		// no free ranges found, so write to the end of the mmap file
		pos.start = b.mmapfEnd
		mmapfNewSize := int64(b.mmapfEnd) + int64(pos.size)
		if err := os.Truncate(b.mmapfPath, mmapfNewSize); err != nil {
			return false, fmt.Errorf("error increasing %s: %w", b.mmapfPath, err)
		}
		b.mmapfEnd = uint64(mmapfNewSize)
	}

	// write to the mmap
	if err := betterbinary.Marshal(evt, b.mmapf[pos.start:]); err != nil {
		return false, fmt.Errorf("error marshaling to %d: %w", pos.start, err)
	}

	// prepare value to be saved in the id index (if we didn't have it already)
	// val: [posb][layerIdRefs...]
	if val == nil {
		val = make([]byte, 12, 12+2*len(b.layers))
		binary.BigEndian.PutUint32(val[0:4], pos.size)
		binary.BigEndian.PutUint64(val[4:12], pos.start)
	}

	// each index that was reserved above for the different layers
	for i, il := range ils {
		iltxn := iltxns[i]

		for k := range il.getIndexKeysForEvent(evt) {
			if err := iltxn.Put(k.dbi, k.key, val[0:12] /* pos */, 0); err != nil {
				b.Logger.Warn().Str("name", il.name).Msg("failed to index event on layer")
			}
		}

		val = binary.BigEndian.AppendUint16(val, il.id)
	}

	// store the id index with the layer references
	if err := mmmtxn.Put(b.indexId, evt.ID[0:8], val, 0); err != nil {
		panic(fmt.Errorf("failed to store %x by id: %w", evt.ID[:], err))
	}

	// msync
	_, _, errno := syscall.Syscall(syscall.SYS_MSYNC,
		uintptr(unsafe.Pointer(&b.mmapf[0])), uintptr(len(b.mmapf)), syscall.MS_SYNC)
	if errno != 0 {
		panic(fmt.Errorf("msync failed: %w", syscall.Errno(errno)))
	}

	return true, nil
}
