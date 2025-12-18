package mmm

import (
	"encoding/binary"
	"time"

	"fiatjaf.com/nostr"
	"github.com/PowerDNS/lmdb-go/lmdb"
)

type EventStats struct {
	Total           uint
	PerWeek         []uint
	PerPubKeyPrefix map[string]PubKeyStats
	PerKind         map[nostr.Kind]KindStats
}

type KindStats struct {
	Total   uint
	PerWeek []uint
}

type PubKeyStats struct {
	Total          uint
	PerWeek        []uint
	PerKind        map[nostr.Kind]uint
	PerKindPerWeek map[nostr.Kind][]uint
}

func (il *IndexingLayer) ComputeStats() (*EventStats, error) {
	stats := &EventStats{
		Total:           0,
		PerWeek:         make([]uint, 0, 24),
		PerPubKeyPrefix: make(map[string]PubKeyStats, 30),
		PerKind:         make(map[nostr.Kind]KindStats, 20),
	}

	err := il.lmdbEnv.View(func(txn *lmdb.Txn) error {
		cursor, err := txn.OpenCursor(il.indexPubkeyKind)
		if err != nil {
			return err
		}
		defer cursor.Close()

		for {
			key, _, err := cursor.Get(nil, nil, lmdb.Next)
			if lmdb.IsNotFound(err) {
				break
			}
			if err != nil {
				return err
			}

			if len(key) < 14 {
				continue
			}

			// parse key: [8 bytes pubkey][2 bytes kind][4 bytes timestamp]
			pubkeyPrefix := nostr.HexEncodeToString(key[0:8])
			kind := nostr.Kind(binary.BigEndian.Uint16(key[8:10]))
			createdTime := time.Unix(int64(binary.BigEndian.Uint32(key[10:14])), 0)

			// figure out how many weeks in the past this is
			weekIndex := weeksInPast(createdTime)

			// update totals
			stats.Total++
			if weekIndex >= 0 {
				for len(stats.PerWeek) <= weekIndex {
					stats.PerWeek = append(stats.PerWeek, 0)
				}
				stats.PerWeek[weekIndex]++
			}
			if this, exists := stats.PerPubKeyPrefix[pubkeyPrefix]; exists {
				this.Total++
				this.PerKind[kind]++
				if weekIndex >= 0 {
					for len(this.PerWeek) <= weekIndex {
						this.PerWeek = append(this.PerWeek, 0)
					}
					this.PerWeek[weekIndex]++
				}
				stats.PerPubKeyPrefix[pubkeyPrefix] = this
			} else {
				stats.PerPubKeyPrefix[pubkeyPrefix] = PubKeyStats{
					Total: 1,
					PerKind: map[nostr.Kind]uint{
						kind: 1,
					},
				}
			}
			if this, exists := stats.PerKind[kind]; exists {
				this.Total++
				if weekIndex >= 0 {
					for len(this.PerWeek) <= weekIndex {
						this.PerWeek = append(this.PerWeek, 0)
					}
					this.PerWeek[weekIndex]++
				}
				stats.PerKind[kind] = this
			} else {
				stats.PerKind[kind] = KindStats{
					Total: 1,
				}
			}
		}

		return nil
	})

	return stats, err
}

func weeksInPast(date time.Time) int {
	now := time.Now()

	if date.After(now) {
		// when in the future always return -1
		return -1
	}

	lastSaturday := now.AddDate(0, 0, -int(now.Weekday()+1))
	lastSaturday = time.Date(lastSaturday.Year(), lastSaturday.Month(), lastSaturday.Day(), 23, 59, 59, 0, lastSaturday.Location())

	// if the date is after the last completed Saturday, it's in the current incomplete week
	if date.After(lastSaturday) {
		return 0
	}

	// calculate the number of complete weeks between the date and last saturday
	daysDiff := int(lastSaturday.Sub(date).Hours() / 24)
	completeWeeks := (daysDiff / 7) + 1 // +1 because we've already passed at least one Saturday

	return completeWeeks
}
