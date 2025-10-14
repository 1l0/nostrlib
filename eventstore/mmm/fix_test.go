package mmm

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"os"
	"testing"

	"fiatjaf.com/nostr"
	"github.com/PowerDNS/lmdb-go/lmdb"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func FuzzRescan(f *testing.F) {
	f.Add(0, uint(3), uint(10), uint(2))
	f.Fuzz(func(t *testing.T, seed int, nlayers, nevents, nbork uint) {
		nlayers = nlayers%5 + 1
		nevents = nevents%100 + 1
		nbork = nbork % nevents

		// create a temporary directory for the test
		tmpDir, err := os.MkdirTemp("", "mmm-rescan-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		logger := zerolog.Nop()
		rnd := rand.New(rand.NewPCG(uint64(seed), 0))

		// initialize MMM
		mmmm := &MultiMmapManager{
			Dir:    tmpDir,
			Logger: &logger,
		}

		err = mmmm.Init()
		require.NoError(t, err)
		defer mmmm.Close()

		// create layers
		for i := range nlayers {
			name := string([]byte{97 + byte(i)})
			il := &IndexingLayer{}
			err = mmmm.EnsureLayer(name, il)
			defer il.Close()
			require.NoError(t, err, "layer %s/%d", name, i)
		}

		// create and store events
		sk := nostr.MustSecretKeyFromHex("945e01e37662430162121b804d3645a86d97df9d256917d86735d0eb219393eb")
		storedEvents := make([]nostr.Event, nevents)

		for i := 0; i < int(nevents); i++ {
			tags := nostr.Tags{}
			// randomly assign to layers
			for j := range nlayers {
				if rnd.UintN(2) == 1 {
					tag := string([]byte{97 + byte(j)})
					tags = append(tags, nostr.Tag{"t", tag})
				}
			}
			if len(tags) == 0 {
				// ensure at least one tag
				tags = append(tags, nostr.Tag{"t", string([]byte{97})})
			}

			evt := nostr.Event{
				CreatedAt: nostr.Timestamp(i),
				Kind:      nostr.KindTextNote,
				Tags:      tags,
				Content:   fmt.Sprintf("test content %d", i),
			}
			evt.Sign(sk)
			storedEvents[i] = evt

			// save to appropriate layers
			for _, layer := range mmmm.layers {
				if evt.Tags.FindWithValue("t", layer.name) != nil {
					err := layer.SaveEvent(evt)
					require.NoError(t, err)
				}
			}
		}

		// get positions of some events to corrupt
		var positionsToBork []position
		var eventsToBork []nostr.Event

		err = mmmm.lmdbEnv.View(func(txn *lmdb.Txn) error {
			cursor, err := txn.OpenCursor(mmmm.indexId)
			require.NoError(t, err)
			defer cursor.Close()

			i := 0
			for key, val, err := cursor.Get(nil, nil, lmdb.First); err == nil && i < int(nbork); key, val, err = cursor.Get(key, val, lmdb.Next) {
				if len(val) >= 12 {
					pos := positionFromBytes(val[0:12])
					positionsToBork = append(positionsToBork, pos)

					// find the corresponding event
					for _, evt := range storedEvents {
						if bytes.Equal(evt.ID[0:8], key) {
							eventsToBork = append(eventsToBork, evt)
							break
						}
					}
					i++
				}
			}
			return nil
		})
		require.NoError(t, err)

		// manually corrupt the mmapped file at these positions
		for _, pos := range positionsToBork {
			// write garbage to the position
			copy(mmmm.mmapf[pos.start:], []byte("CORRUPTED_DATA_XXXX"))
		}

		// call Rescan and check that borked events are removed
		err = mmmm.Rescan()
		require.NoError(t, err)

		// verify borked events are no longer accessible
		for _, evt := range eventsToBork {
			gotEvt, layers := mmmm.GetByID(evt.ID)
			require.Nil(t, gotEvt, "borked event should be removed")
			require.Empty(t, layers, "borked event should have no layer references")
		}

		// Test that non-borked events are still accessible
		for _, evt := range storedEvents {
			found := false
			for _, borkedEvt := range eventsToBork {
				if bytes.Equal(evt.ID[:], borkedEvt.ID[:]) {
					found = true
					break
				}
			}
			if !found {
				// this event should still be accessible
				gotEvt, layers := mmmm.GetByID(evt.ID)
				require.NotNil(t, gotEvt, "non-borked event should still exist")
				require.NotEmpty(t, layers, "non-borked event should have layer references")
			}
		}
	})
}
