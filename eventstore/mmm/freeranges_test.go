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
			for range rnd.IntN(40) {
				evt := nostr.Event{
					CreatedAt: nostr.Timestamp(rnd.Uint32()),
					Kind:      1,
					Content:   strings.Repeat("z", rnd.IntN(1000)),
				}
				evt.Sign(sk)
				err := il.SaveEvent(evt)
				require.NoError(t, err)

				total++
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
			t.Logf("loop -- current %d", total)

			if chance(30) {
				break
			}
		}
	})
}
