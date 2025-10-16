package mmm

import (
	"math/rand/v2"
	"os"
	"strings"
	"testing"

	"fiatjaf.com/nostr"
	"github.com/PowerDNS/lmdb-go/lmdb"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func FuzzFreeRanges(f *testing.F) {
	f.Add(0)
	f.Fuzz(func(t *testing.T, seed int) {
		// create a temporary directory for the test
		tmpDir, err := os.MkdirTemp("", "mmm-freeranges-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		logger := zerolog.Nop()
		rnd := rand.New(rand.NewPCG(uint64(seed), 0))
		chance := func(n uint) bool {
			return rnd.UintN(100) < n
		}

		// initialize MMM
		mmmm := &MultiMmapManager{
			Dir:    tmpDir,
			Logger: &logger,
		}

		err = mmmm.Init()
		require.NoError(t, err)
		defer mmmm.Close()

		// create a single layer
		il, err := mmmm.EnsureLayer("a")
		require.NoError(t, err)
		defer il.Close()

		sk := nostr.MustSecretKeyFromHex("945e01e37662430162121b804d3645a86d97df9d256917d86735d0eb219393eb")

		total := 0
		for {
			freeBefore := countUsableFreeRanges(mmmm)

			for i := range rnd.IntN(40) {
				content := "1" // ensure at least one event is as small as it can be
				if i > 0 {
					strings.Repeat("z", rnd.IntN(1000))
				}

				evt := nostr.Event{
					CreatedAt: nostr.Timestamp(rnd.Uint32()),
					Kind:      1,
					Content:   content,
					Tags:      nostr.Tags{},
				}
				evt.Sign(sk)
				err := il.SaveEvent(evt)
				require.NoError(t, err)

				total++
			}

			freeAfter := countUsableFreeRanges(mmmm)
			if freeBefore > 0 {
				require.Lessf(t, freeAfter, freeBefore, "must use some of the existing free ranges when inserting new events")
			}

			// delete some events
			if total > 0 {
				for range rnd.IntN(total) {
					for evt := range il.QueryEvents(nostr.Filter{}, 1) {
						err := il.DeleteEvent(evt.ID)
						require.NoError(t, err)

						total--
					}
				}
			}

			mmmm.lmdbEnv.View(func(txn *lmdb.Txn) error {
				expectedFreeRanges, _ := mmmm.gatherFreeRanges(txn)
				require.Equalf(t, expectedFreeRanges, mmmm.freeRanges, "expected %s, got %s", expectedFreeRanges, mmmm.freeRanges)
				return nil
			})

			if chance(20) {
				break
			}
		}
	})
}

func countUsableFreeRanges(mmmm *MultiMmapManager) int {
	count := 0
	for _, fr := range mmmm.freeRanges {
		if fr.size > 150 {
			count++
		}
	}
	return count
}
