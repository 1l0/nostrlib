package badger

import (
	"cmp"
	"encoding/binary"
	"fmt"
	"slices"
	"testing"
	"time"

	"fiatjaf.com/nostr"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

func FuzzQuery(f *testing.F) {
	f.Add(uint(200), uint(50), uint(13), uint(2), uint(2), uint(0), uint(1))
	f.Fuzz(func(t *testing.T, total, limit, authors, timestampAuthorFactor, seedFactor, kinds, kindFactor uint) {
		total++
		authors++
		seedFactor++
		kindFactor++
		if kinds == 1 {
			kinds++
		}
		if limit == 0 {
			return
		}

		// ~ setup db

		bdb, err := badger.Open(badger.DefaultOptions("").WithInMemory(true))
		if err != nil {
			t.Fatalf("failed to create database: %s", err)
			return
		}
		db := &BadgerBackend{}
		db.DB = bdb

		if err := db.runMigrations(); err != nil {
			t.Fatalf("error: %s", err)
			return
		}

		if err := db.DB.View(func(txn *badger.Txn) error {
			it := txn.NewIterator(badger.IteratorOptions{
				Prefix:  []byte{0},
				Reverse: true,
			})
			it.Seek([]byte{1})
			if it.Valid() {
				key := it.Item().Key()
				idx := key[1:]
				serial := binary.BigEndian.Uint32(idx)
				db.serial.Store(serial)
			}
			it.Close()
			return nil
		}); err != nil {
			t.Fatalf("failed to initialize serial: %s", err)
			return
		}

		db.MaxLimit = 500
		defer db.Close()

		// ~ start actual test

		filter := nostr.Filter{
			Authors: make([]nostr.PubKey, authors),
			Limit:   int(limit),
		}
		var maxKind uint16 = 1
		if kinds > 0 {
			filter.Kinds = make([]uint16, kinds)
			for i := range filter.Kinds {
				filter.Kinds[i] = uint16(kindFactor) * uint16(i)
			}
			maxKind = filter.Kinds[len(filter.Kinds)-1]
		}

		for i := 0; i < int(authors); i++ {
			sk := nostr.SecretKey{}
			binary.BigEndian.PutUint32(sk[:], uint32(i%int(authors*seedFactor))+1)
			pk := nostr.GetPublicKey(sk)
			filter.Authors[i] = pk
		}

		expected := make([]nostr.Event, 0, total)
		for i := 0; i < int(total); i++ {
			skseed := uint32(i%int(authors*seedFactor)) + 1
			sk := nostr.SecretKey{}
			binary.BigEndian.PutUint32(sk[:], skseed)

			evt := nostr.Event{
				CreatedAt: nostr.Timestamp(skseed)*nostr.Timestamp(timestampAuthorFactor) + nostr.Timestamp(i),
				Content:   fmt.Sprintf("unbalanced %d", i),
				Tags:      nostr.Tags{},
				Kind:      uint16(i) % maxKind,
			}
			err := evt.Sign(sk)
			require.NoError(t, err)

			err = db.SaveEvent(evt)
			require.NoError(t, err)

			if filter.Matches(evt) {
				expected = append(expected, evt)
			}
		}

		slices.SortFunc(expected, nostr.CompareEventReverse)
		if len(expected) > int(limit) {
			expected = expected[0:limit]
		}

		start := time.Now()
		// fmt.Println(filter)
		res := slices.Collect(db.QueryEvents(filter))
		end := time.Now()

		require.NoError(t, err)
		require.Equal(t, len(expected), len(res), "number of results is different than expected")

		require.Less(t, end.Sub(start).Milliseconds(), int64(1500), "query took too long")
		require.True(t, slices.IsSortedFunc(res, func(a, b nostr.Event) int { return cmp.Compare(b.CreatedAt, a.CreatedAt) }), "results are not sorted")

		nresults := len(expected)

		getTimestamps := func(events []nostr.Event) []nostr.Timestamp {
			res := make([]nostr.Timestamp, len(events))
			for i, evt := range events {
				res[i] = evt.CreatedAt
			}
			return res
		}

		// fmt.Println(" expected   result")
		// for i := range expected {
		// 	fmt.Println(" ", expected[i].CreatedAt, expected[i].ID[0:8], "  ", res[i].CreatedAt, res[i].ID[0:8], "           ", i)
		// }

		require.Equal(t, expected[0].CreatedAt, res[0].CreatedAt, "first result is wrong")
		require.Equal(t, expected[nresults-1].CreatedAt, res[nresults-1].CreatedAt, "last result is wrong")
		require.Equal(t, getTimestamps(expected), getTimestamps(res))

		for _, evt := range res {
			require.True(t, filter.Matches(evt), "event %s doesn't match filter %s", evt, filter)
		}
	})
}
