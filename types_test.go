package nostr

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIDJSONEncoding(t *testing.T) {
	id := MustIDFromHex("abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789")

	// test marshaling
	b, err := json.Marshal(id)
	require.NoError(t, err)
	require.Equal(t, `"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"`, string(b))

	// test unmarshaling
	var id2 ID
	err = json.Unmarshal(b, &id2)
	require.NoError(t, err)
	require.Equal(t, id, id2)

	// test unmarshaling invalid json
	err = json.Unmarshal([]byte(`"not64chars"`), &id2)
	require.Error(t, err)

	// test unmarshaling invalid hex
	err = json.Unmarshal([]byte(`"zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"`), &id2)
	require.Error(t, err)
}

func TestPubKeyJSONEncoding(t *testing.T) {
	pk := MustPubKeyFromHex("abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789")

	// test marshaling
	b, err := json.Marshal(pk)
	require.NoError(t, err)
	require.Equal(t, `"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"`, string(b))

	// test unmarshaling
	var pk2 PubKey
	err = json.Unmarshal(b, &pk2)
	require.NoError(t, err)
	require.Equal(t, pk, pk2)

	// test unmarshaling invalid json
	err = json.Unmarshal([]byte(`"not64chars"`), &pk2)
	require.Error(t, err)

	// test unmarshaling invalid hex
	err = json.Unmarshal([]byte(`"zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"`), &pk2)
	require.Error(t, err)
}

type TestStruct struct {
	ID     ID     `json:"id"`
	PubKey PubKey `json:"pubkey"`
	Name   string `json:"name"`
}

func TestStructWithIDAndPubKey(t *testing.T) {
	ts := TestStruct{
		ID:     MustIDFromHex("abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"),
		PubKey: MustPubKeyFromHex("123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0"),
		Name:   "test",
	}

	// test marshaling
	b, err := json.Marshal(ts)
	require.NoError(t, err)
	require.Equal(t, `{"id":"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789","pubkey":"123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0","name":"test"}`, string(b))

	// test unmarshaling
	var ts2 TestStruct
	err = json.Unmarshal(b, &ts2)
	require.NoError(t, err)
	require.Equal(t, ts, ts2)

	// test unmarshaling with missing fields
	var ts3 TestStruct
	err = json.Unmarshal([]byte(`{"name":"test"}`), &ts3)
	require.NoError(t, err)
	require.Equal(t, "test", ts3.Name)
	require.Equal(t, ZeroID, ts3.ID)
	require.Equal(t, ZeroPK, ts3.PubKey)

	// test unmarshaling with invalid ID
	err = json.Unmarshal([]byte(`{"id":"invalid","pubkey":"123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0","name":"test"}`), &ts2)
	require.Error(t, err)

	// test unmarshaling with invalid PubKey
	err = json.Unmarshal([]byte(`{"id":"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789","pubkey":"invalid","name":"test"}`), &ts2)
	require.Error(t, err)
}
