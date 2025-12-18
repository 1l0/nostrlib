//go:build tinygo

package nip05

import (
	json "encoding/json"
)

func (v WellKnownResponse) Marshal() ([]byte, error) {
	return json.Marshal(v)
}

func (v WellKnownResponse) Unmarshal(data []byte) error {
	return json.Unmarshal(data, v)
}
