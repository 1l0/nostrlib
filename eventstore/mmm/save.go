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

	// prepare transactions
	mmmtxn, err := il.mmmm.lmdbEnv.BeginTxn(nil, 0)
	if err != nil {
		return err
	}
	defer func() {
		// defer abort but only if we haven't committed (we'll set it to nil after committing)
		if mmmtxn != nil {
			mmmtxn.Abort()
		}
	}()
	mmmtxn.RawRead = true

	iltxn, err := il.lmdbEnv.BeginTxn(nil, 0)
	if err != nil {
		return err
	}
	defer func() {
		// defer abort but only if we haven't committed (we'll set it to nil after committing)
		if iltxn != nil {
			iltxn.Abort()
		}
	}()
	iltxn.RawRead = true

	// the actual save operation
	if _, err := il.mmmm.storeOn(mmmtxn, iltxn, il, evt); err != nil {
		return err
	}

	// commit in this order to minimize problematic inconsistencies
	if err := mmmtxn.Commit(); err != nil {
		return fmt.Errorf("can't commit mmmtxn: %w", err)
	}
	mmmtxn = nil
	if err := iltxn.Commit(); err != nil {
		return fmt.Errorf("can't commit iltxn: %w", err)
	}
	iltxn = nil

	return nil
}

func (b *MultiMmapManager) storeOn(
	mmmtxn *lmdb.Txn,
	iltxn *lmdb.Txn,
	il *IndexingLayer,
	evt nostr.Event,
) (stored bool, err error) {
	// check if we already have this id
	var pos position
	val, err := mmmtxn.Get(b.indexId, evt.ID[0:8])
	if err == nil {
		pos = positionFromBytes(val[0:12])
		// we found the event, now check if it is already indexed by the layer that wants to store it
		for s := 12; s < len(val); s += 2 {
			ilid := binary.BigEndian.Uint16(val[s : s+2])
			if il.id == ilid {
				// already on the specified layer, we can end here
				return false, nil
			}
		}
	} else if !lmdb.IsNotFound(err) {
		// if we got an error from lmdb we will only proceed if it's NotFound -- for anything else we will error
		return false, fmt.Errorf("error checking existence: %w", err)
	}

	// ok, now we have to write the event to the mmapped file
	// unless we already have the event stored, in that case we don't have to write it again, we'll just reuse it
	if val == nil {
		// get event binary size
		pos = position{
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
				// (in case of conflict we lose this free range but it's ok, it will be recovered on the next startup)
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

		// msync
		_, _, errno := syscall.Syscall(syscall.SYS_MSYNC,
			uintptr(unsafe.Pointer(&b.mmapf[0])), uintptr(len(b.mmapf)), syscall.MS_SYNC)
		if errno != 0 {
			panic(fmt.Errorf("msync failed: %w", syscall.Errno(errno)))
		}

		// prepare value to be saved in the id index (if we didn't have it already)
		// val: [posb][layerIdRefs...]
		val = make([]byte, 12, 12+2) // only reserve room for one layer after the position
		writeBytesFromPosition(val, pos)
	}

	// generate and save indexes
	for k := range il.getIndexKeysForEvent(evt) {
		if err := iltxn.Put(k.dbi, k.key, val[0:12] /* pos */, 0); err != nil {
			b.Logger.Warn().Str("name", il.name).Msg("failed to index event on layer")
		}
	}

	// add layer to the id index val
	val = binary.BigEndian.AppendUint16(val, il.id)

	// store the id index with the new layer reference
	if err := mmmtxn.Put(b.indexId, evt.ID[0:8], val, 0); err != nil {
		return false, fmt.Errorf("failed to store %x by id: %w", evt.ID[:], err)
	}

	return true, nil
}
