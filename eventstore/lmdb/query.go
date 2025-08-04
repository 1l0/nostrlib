package lmdb

import (
	"iter"
	"log"
	"math"
	"slices"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/eventstore/codec/betterbinary"
	"fiatjaf.com/nostr/eventstore/internal"
	"github.com/PowerDNS/lmdb-go/lmdb"
)

func (b *LMDBBackend) QueryEvents(filter nostr.Filter, maxLimit int) iter.Seq[nostr.Event] {
	return func(yield func(nostr.Event) bool) {
		if filter.Search != "" {
			return
		}

		if filter.IDs != nil {
			// do a special id query
			// TODO
		}

		// max number of events we'll return
		if tlimit := filter.GetTheoreticalLimit(); tlimit == 0 || filter.LimitZero {
			return
		} else if tlimit < maxLimit {
			maxLimit = tlimit
		}
		if filter.Limit > 0 && filter.Limit < maxLimit {
			maxLimit = filter.Limit
		}

		b.lmdbEnv.View(func(txn *lmdb.Txn) error {
			txn.RawRead = true
			return b.query(txn, filter, maxLimit, yield)
		})
	}
}

func (b *LMDBBackend) query(txn *lmdb.Txn, filter nostr.Filter, limit int, yield func(nostr.Event) bool) error {
	queries, extraAuthors, extraKinds, extraTagKey, extraTagValues, since, err := b.prepareQueries(filter)
	if err != nil {
		return err
	}

	iterators := make(iterators, len(queries))
	batchSizePerQuery := internal.BatchSizePerNumberOfQueries(limit, len(queries))

	for q, query := range queries {
		cursor, err := txn.OpenCursor(queries[q].dbi)
		if err != nil {
			return err
		}
		iterators[q] = iterator{
			query:  query,
			cursor: cursor,
		}

		defer cursor.Close()
		iterators[q].seek(queries[q].startingPoint)
	}

	// initial pull from all queries
	for _, it := range iterators {
		it.pull(batchSizePerQuery, since)
	}

	numberOfIteratorsToPullOnEachRound := max(1, int(math.Ceil(float64(len(iterators))/float64(14))))
	totalEventsEmitted := 0
	tempResults := make([]nostr.Event, 0, batchSizePerQuery*2)

	for len(iterators) > 0 {
		// reset stuff
		tempResults = tempResults[:0]

		// after pulling from all iterators once we now find out what iterators are
		// the ones we should keep pulling from next (i.e. which one's last emitted timestamp is the highest)
		iterators.quickselect(min(numberOfIteratorsToPullOnEachRound, len(iterators)), 0, len(iterators))

		// we now know what is our threshold
		threshold := iterators[len(iterators)-1].last
		// so we can emit all the events higher than it
		for _, it := range iterators {
			for t, ts := range it.timestamps {
				if ts >= threshold {
					idx := it.idxs[t]

					// discard this regardless of what happens
					internal.SwapDelete(it.timestamps, t)
					internal.SwapDelete(it.idxs, t)
					t--

					// fetch actual event
					bin, err := txn.Get(b.rawEventStore, idx)
					if err != nil {
						log.Printf("lmdb: failed to get %x from raw event store: %s (query prefix=%x, index=%s)\n",
							idx, err, it.query.prefix, b.dbiName(it.query.dbi))
						continue
					}

					// check it against pubkeys without decoding the entire thing
					if extraAuthors != nil && !slices.Contains(extraAuthors, betterbinary.GetPubKey(bin)) {
						continue
					}

					// check it against kinds without decoding the entire thing
					if extraKinds != nil && !slices.Contains(extraKinds, betterbinary.GetKind(bin)) {
						continue
					}

					// decode the entire thing
					event := nostr.Event{}
					if err := betterbinary.Unmarshal(bin, &event); err != nil {
						log.Printf("lmdb: value read error (id %x) on query prefix %x sp %x dbi %s: %s\n",
							betterbinary.GetID(bin), it.query.prefix, it.query.startingPoint, b.dbiName(it.query.dbi), err)
						continue
					}

					// fmt.Println("      event", betterbinary.GetID(bin), "kind", betterbinary.GetKind(bin).Num(), "author", betterbinary.GetPubKey(bin), "ts", betterbinary.GetCreatedAt(bin), hex.EncodeToString(it.key), it.valIdx)

					// if there is still a tag to be checked, do it now
					if extraTagValues != nil && !event.Tags.ContainsAny(extraTagKey, extraTagValues) {
						continue
					}

					tempResults = append(tempResults, event)
				}
			}
		}

		// emit this stuff in order
		slices.SortFunc(tempResults, nostr.CompareEvent)
		for _, evt := range tempResults {
			if !yield(evt) {
				return nil
			}

			totalEventsEmitted++
			if totalEventsEmitted == limit {
				return nil
			}
		}

		// now pull more events
		for i := 0; i < min(len(iterators), numberOfIteratorsToPullOnEachRound); i++ {
			it := iterators[i]
			if it.exhausted {
				if len(it.idxs) == 0 {
					// eliminating this from the list of iterators
					internal.SwapDelete(iterators, i)
					i--
				}
				continue
			}

			it.pull(batchSizePerQuery, since)
		}
	}

	return nil
}
