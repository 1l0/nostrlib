//go:build tinygo

package nostr

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func mustRelayConnect(t *testing.T, url string) *Relay {
	t.Helper()
	rl, err := RelayConnect(context.Background(), url, RelayOptions{})
	require.NoError(t, err)

	return rl
}
