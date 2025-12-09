package sdk

import (
	"context"
	"testing"
	"time"

	"fiatjaf.com/nostr"
	"github.com/stretchr/testify/require"
)

func TestFetchZapProvider(t *testing.T) {
	sys := NewSystem()
	ctx := context.Background()

	pk, err := nostr.PubKeyFromHex("fa984bd7dbb282f07e16e7ae87b26a2a7b9b90b7246a44771f0cf5ae58018f52")
	require.NoError(t, err)

	// inject metadata to avoid network fetch which crashes in tinygo/wasm environment due to websocket issues
	sys.MetadataCache.SetWithTTL(pk, ProfileMetadata{
		PubKey: pk,
		LUD16:  "pablof7z@primal.net",
	}, time.Hour)

	zp := sys.FetchZapProvider(ctx, pk)
	expected, err := nostr.PubKeyFromHex("f81611363554b64306467234d7396ec88455707633f54738f6c4683535098cd3")
	require.NoError(t, err)
	require.Equal(t, expected, zp)
}
