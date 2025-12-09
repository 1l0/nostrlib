//go:build !tinygo

package nostr

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func mustRelayConnect(t *testing.T, url string) *Relay {
	t.Helper()
	rl, err := RelayConnect(t.Context(), url, RelayOptions{})
	require.NoError(t, err)

	return rl
}
