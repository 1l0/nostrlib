package nostr

import (
	stdjson "encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func makeKeyPair(t *testing.T) (priv, pub [32]byte) {
	t.Helper()

	privkey := Generate()
	pubkey := GetPublicKey(privkey)

	return privkey, pubkey
}

func mustRelayConnect(t *testing.T, url string) *Relay {
	t.Helper()

	rl, err := RelayConnect(t.Context(), url, RelayOptions{})
	require.NoError(t, err)

	return rl
}

func parseEventMessage(t *testing.T, raw []stdjson.RawMessage) Event {
	t.Helper()

	require.Condition(t, func() (success bool) {
		return len(raw) >= 2
	})

	var typ string
	err := json.Unmarshal(raw[0], &typ)
	require.NoError(t, err)
	require.Equal(t, "EVENT", typ)

	var event Event
	err = json.Unmarshal(raw[1], &event)
	require.NoError(t, err)

	return event
}

func parseSubscriptionMessage(t *testing.T, raw []stdjson.RawMessage) (subid string, filters []Filter) {
	t.Helper()

	require.Greater(t, len(raw), 3)

	var typ string
	err := json.Unmarshal(raw[0], &typ)

	require.NoError(t, err)
	require.Equal(t, "REQ", typ)

	var id string
	err = json.Unmarshal(raw[1], &id)
	require.NoError(t, err)

	var ff []Filter
	for _, b := range raw[2:] {
		var f Filter
		err := json.Unmarshal(b, &f)
		require.NoError(t, err)
		ff = append(ff, f)
	}
	return id, ff
}
