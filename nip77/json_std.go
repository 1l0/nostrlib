//go:build !tinygo

package nip77

import (
	"bytes"

	"fiatjaf.com/nostr"
	"github.com/mailru/easyjson"
	jwriter "github.com/mailru/easyjson/jwriter"
)

func unmarshalFilter(data []byte, v *nostr.Filter) error {
	return easyjson.Unmarshal(data, v)
}

func writeFilterToBuffer(buf *bytes.Buffer, f nostr.Filter) error {
	w := jwriter.Writer{NoEscapeHTML: true}
	f.MarshalEasyJSON(&w)
	if w.Error != nil {
		return w.Error
	}
	w.Buffer.DumpTo(buf)
	return nil
}
