package mmm

import (
	"encoding/binary"
	"slices"

	"fiatjaf.com/nostr"
	"github.com/PowerDNS/lmdb-go/lmdb"
)

func (b *MultiMmapManager) Rescan() error {
	b.writeMutex.Lock()
	defer b.writeMutex.Unlock()

	return b.lmdbEnv.Update(func(mmmtxn *lmdb.Txn) error {
		cursor, err := mmmtxn.OpenCursor(b.indexId)
		if err != nil {
			panic(err)
		}
		defer cursor.Close()

		type entry struct {
			idPrefix []byte
			pos      position
		}
		var toPurge []entry

		for key, val, err := cursor.Get(nil, nil, lmdb.First); err == nil; key, val, err = cursor.Get(key, val, lmdb.Next) {
			pos := positionFromBytes(val[0:12])

			// for every event in this index
			var borked bool
			var ts nostr.Timestamp
			// we try to load it
			if ts_, err := b.loadJustTimestamp(pos); err == nil {
				// if we succeed we assume the event is ok for now
				borked = false
				ts = ts_
			} else {
				// otherwise we know it's borked
				borked = true
			}

			var evt *nostr.Event
			var layersToRemove []uint16

			// then for every layer referenced in there we check
			for s := 12; s < len(val); s += 2 {
				layerId := binary.BigEndian.Uint16(val[s : s+2])
				layer := b.layers.ByID(layerId)
				if layer == nil {
					continue
				}

				layer.lmdbEnv.View(func(txn *lmdb.Txn) error {
					txn.RawRead = true

					if borked {
						// for borked events we have to do a bruteforce check
						if layer.hasAtPosition(txn, pos) {
							// expected -- delete anyway since it's borked
							if err := layer.bruteDeleteIndexes(txn, pos); err != nil {
								panic(err)
							}
						} else {
							// this stuff is doubly borked -- let's do nothing
							return nil
						}
					} else {
						// otherwise we do a more reasonable check
						if layer.hasAtTimestampAndPosition(txn, ts, pos) {
							// expected, all good
						} else {
							// can't find it in this layer, so update source reference to remove this
							// and clear it from this layer (if any traces remain)
							if evt == nil {
								if err := b.loadEvent(pos, evt); err != nil {
									// can't load event, means it's borked
									borked = true

									// act as if it's borked
									if err := layer.bruteDeleteIndexes(txn, pos); err != nil {
										panic(err)
									}
								} else {
									goto haveEvent
								}
							}

						haveEvent:
							if err := layer.deleteIndexes(txn, *evt, val[0:12]); err != nil {
								panic(err)
							}

							// we'll remove references to this later
							// (no need to do anything in the borked case as everything will be deleted)
							layersToRemove = append(layersToRemove, layerId)
						}
					}

					return nil
				})
			}

			if borked {
				toPurge = append(toPurge, entry{idPrefix: key, pos: pos})
			} else if len(layersToRemove) > 0 {
				for s := 12; s < len(val); {
					if slices.Contains(layersToRemove, binary.BigEndian.Uint16(val[s:s+2])) {
						// swap-delete
						copy(val[s:s+2], val[len(val)-2:])
						val = val[len(val)-2:]
					} else {
						s += 2
					}
				}

				if len(val) > 12 {
					if err := mmmtxn.Put(b.indexId, key, val, 0); err != nil {
						panic(err)
					}
				} else {
					toPurge = append(toPurge, entry{idPrefix: key, pos: pos})
				}
			}
		}

		for _, entry := range toPurge {
			if err := b.purge(mmmtxn, entry.idPrefix, entry.pos); err != nil {
				panic(err)
			}
		}

		return nil
	})
}

func (il *IndexingLayer) hasAtTimestampAndPosition(iltxn *lmdb.Txn, ts nostr.Timestamp, pos position) (exists bool) {
	cursor, err := iltxn.OpenCursor(il.indexCreatedAt)
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key[0:4], uint32(ts))

	if _, val, err := cursor.Get(key, nil, lmdb.SetKey); err == nil {
		if positionFromBytes(val[0:12]) == pos {
			exists = true
		}
	}

	return exists
}

func (il *IndexingLayer) hasAtPosition(iltxn *lmdb.Txn, pos position) (exists bool) {
	cursor, err := iltxn.OpenCursor(il.indexCreatedAt)
	if err != nil {
		panic(err)
	}
	defer cursor.Close()

	for key, val, err := cursor.Get(nil, nil, lmdb.First); err == nil; key, val, err = cursor.Get(key, val, lmdb.Next) {
		if positionFromBytes(val[0:12]) == pos {
			exists = true
			break
		}
	}

	return exists
}

func (il *IndexingLayer) bruteDeleteIndexes(iltxn *lmdb.Txn, pos position) error {
	type entry struct {
		key []byte
		val []byte
	}

	toDelete := make([]entry, 0, 8)

	for _, index := range []lmdb.DBI{
		il.indexCreatedAt,
		il.indexKind,
		il.indexPubkey,
		il.indexPubkeyKind,
		il.indexPTagKind,
		il.indexTag,
		il.indexTag32,
		il.indexTagAddr,
	} {
		cursor, err := iltxn.OpenCursor(index)
		if err != nil {
			panic(err)
		}
		defer cursor.Close()

		for key, val, err := cursor.Get(nil, nil, lmdb.First); err == nil; key, val, err = cursor.Get(key, val, lmdb.Next) {
			if positionFromBytes(val[0:12]) == pos {
				toDelete = append(toDelete, entry{key, val})
			}
		}

		for _, entry := range toDelete {
			if err := iltxn.Del(index, entry.key, entry.val); err != nil {
				return err
			}
		}

		toDelete = toDelete[:0]
	}

	return nil
}
