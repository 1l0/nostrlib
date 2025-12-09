//go:build tinygo

package nostr

import (
	"encoding/hex"
	"strconv"
)

func (evt Event) String() string {
	j, _ := json.Marshal(evt)
	return string(j)
}

func (evt Event) MarshalJSON() ([]byte, error) {
	type EventJSON struct {
		Kind      Kind      `json:"kind"`
		ID        ID        `json:"id"`
		PubKey    PubKey    `json:"pubkey"`
		CreatedAt Timestamp `json:"created_at"`
		Tags      Tags      `json:"tags"`
		Content   string    `json:"content"`
		Sig       string    `json:"sig"`
	}

	ej := EventJSON{
		Kind:      evt.Kind,
		ID:        evt.ID,
		PubKey:    evt.PubKey,
		CreatedAt: evt.CreatedAt,
		Tags:      evt.Tags,
		Content:   evt.Content,
		Sig:       hex.EncodeToString(evt.Sig[:]),
	}

	return json.Marshal(ej)
}

func (evt *Event) UnmarshalJSON(data []byte) error {
	type EventJSON struct {
		Kind      Kind      `json:"kind"`
		ID        ID        `json:"id"`
		PubKey    PubKey    `json:"pubkey"`
		CreatedAt Timestamp `json:"created_at"`
		Tags      Tags      `json:"tags"`
		Content   string    `json:"content"`
		Sig       string    `json:"sig"`
	}

	var ej EventJSON
	if err := json.Unmarshal(data, &ej); err != nil {
		return err
	}

	evt.Kind = ej.Kind
	evt.ID = ej.ID
	evt.PubKey = ej.PubKey
	evt.CreatedAt = ej.CreatedAt
	evt.Tags = ej.Tags
	evt.Content = ej.Content

	if len(ej.Sig) == 128 {
		b, err := hex.DecodeString(ej.Sig)
		if err != nil {
			return err
		}
		copy(evt.Sig[:], b)
	}

	return nil
}

// Serialize outputs a byte array that can be hashed to produce the canonical event "id".
func (evt Event) Serialize() []byte {
	// the serialization process is just putting everything into a JSON array
	// so the order is kept. See NIP-01
	dst := make([]byte, 4+64, 100+len(evt.Content)+len(evt.Tags)*80)

	// the header portion is easy to serialize
	// [0,"pubkey",created_at,kind,[
	copy(dst, `[0,"`)
	hex.Encode(dst[4:4+64], evt.PubKey[:]) // there will always be such capacity
	dst = append(dst, `",`...)
	dst = append(dst, strconv.FormatInt(int64(evt.CreatedAt), 10)...)
	dst = append(dst, `,`...)
	dst = append(dst, strconv.FormatUint(uint64(evt.Kind), 10)...)
	dst = append(dst, `,`...)

	// tags
	dst = append(dst, '[')
	for i, tag := range evt.Tags {
		if i > 0 {
			dst = append(dst, ',')
		}
		// tag item
		dst = append(dst, '[')
		for i, s := range tag {
			if i > 0 {
				dst = append(dst, ',')
			}
			dst = escapeString(dst, s)
		}
		dst = append(dst, ']')
	}
	dst = append(dst, "],"...)

	// content needs to be escaped in general as it is user generated.
	dst = escapeString(dst, evt.Content)
	dst = append(dst, ']')

	return dst
}
