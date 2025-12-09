//go:build tinygo

package nip77

import (
	"bytes"
	"encoding/json"

	"fiatjaf.com/nostr"
)

func unmarshalFilter(data []byte, v *nostr.Filter) error {
	return json.Unmarshal(data, v)
}

func writeFilterToBuffer(buf *bytes.Buffer, f nostr.Filter) error {
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	buf.Write(b)
	return nil
}
