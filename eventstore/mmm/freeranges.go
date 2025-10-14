package mmm

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/PowerDNS/lmdb-go/lmdb"
)

func (b *MultiMmapManager) GatherFreeRanges(txn *lmdb.Txn) error {
	cursor, err := txn.OpenCursor(b.indexId)
	if err != nil {
		return fmt.Errorf("failed to open cursor on indexId: %w", err)
	}
	defer cursor.Close()

	usedPositions := make([]position, 0, 256)
	for key, val, err := cursor.Get(nil, nil, lmdb.First); err == nil; key, val, err = cursor.Get(key, val, lmdb.Next) {
		pos := positionFromBytes(val[0:12])
		usedPositions = append(usedPositions, pos)
	}

	// sort used positions by start
	slices.SortFunc(usedPositions, func(a, b position) int { return cmp.Compare(a.start, b.start) })

	// calculate free ranges as gaps between used positions
	b.freeRanges = make([]position, 0, len(usedPositions)/2)
	var currentStart uint64 = 0
	for _, pos := range usedPositions {
		if pos.start > currentStart {
			// gap from currentStart to pos.start
			freeSize := pos.start - currentStart
			if freeSize > 0 {
				b.freeRanges = append(b.freeRanges, position{
					start: currentStart,
					size:  uint32(freeSize),
				})
			}
		}
		currentStart = pos.start + uint64(pos.size)
	}

	// sort free ranges by size (smallest first, as before)
	slices.SortFunc(b.freeRanges, func(a, b position) int { return cmp.Compare(a.size, b.size) })

	logOp := b.Logger.Debug()
	for _, pos := range b.freeRanges {
		if pos.size > 20 {
			logOp = logOp.Uint32(fmt.Sprintf("%d", pos.start), pos.size)
		}
	}
	logOp.Msg("calculated free ranges from index scan")

	return nil
}

func (b *MultiMmapManager) mergeNewFreeRange(pos position) (isAtEnd bool) {
	// before adding check if we can merge this with some other range
	// (to merge means to delete the previous and add a new one)
	toDelete := make([]int, 0, 2)
	for f, fr := range b.freeRanges {
		if pos.start+uint64(pos.size) == fr.start {
			// [new_pos_to_be_freed][existing_fr] -> merge!
			toDelete = append(toDelete, f)
			pos.size = pos.size + fr.size
		} else if fr.start+uint64(fr.size) == pos.start {
			// [existing_fr][new_pos_to_be_freed] -> merge!
			toDelete = append(toDelete, f)
			pos.start = fr.start
			pos.size = fr.size + pos.size
		}
	}
	slices.SortFunc(toDelete, func(a, b int) int { return b - a })
	for _, idx := range toDelete {
		b.freeRanges = slices.Delete(b.freeRanges, idx, idx+1)
	}

	// when we're at the end of a file we just delete everything and don't add new free ranges
	// the caller will truncate the mmap file and adjust the position accordingly
	if pos.start+uint64(pos.size) == b.mmapfEnd {
		return true
	}

	b.addNewFreeRange(pos)
	return false
}

func (b *MultiMmapManager) addNewFreeRange(pos position) {
	// update freeranges slice in memory
	idx, _ := slices.BinarySearchFunc(b.freeRanges, pos, func(item, target position) int {
		if item.size > target.size {
			return 1
		} else if target.size > item.size {
			return -1
		} else if item.start > target.start {
			return 1
		} else {
			return -1
		}
	})
	b.freeRanges = slices.Insert(b.freeRanges, idx, pos)
}
