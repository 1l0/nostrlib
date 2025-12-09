//go:build tinygo

package nip46

import (
	stdjson "encoding/json"
	"io"
)

var json = stdJsonWrapper{}

type stdJsonWrapper struct{}

func (stdJsonWrapper) Marshal(v any) ([]byte, error) {
	return stdjson.Marshal(v)
}

func (stdJsonWrapper) Unmarshal(data []byte, v any) error {
	return stdjson.Unmarshal(data, v)
}

func (stdJsonWrapper) NewEncoder(w io.Writer) *stdjson.Encoder {
	return stdjson.NewEncoder(w)
}

func (stdJsonWrapper) NewDecoder(r io.Reader) *stdjson.Decoder {
	return stdjson.NewDecoder(r)
}
