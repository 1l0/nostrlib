//go:build tinygo

package nostr

import (
	"encoding/hex"
	"strconv"

	"github.com/tidwall/gjson"
)

func (evt Event) String() string {
	j, _ := json.Marshal(evt)
	return string(j)
}

func (evt Event) MarshalJSON() ([]byte, error) {
	var b []byte
	b = append(b, `{"kind":`...)
	b = append(b, strconv.Itoa(int(evt.Kind))...)
	if evt.ID != ZeroID {
		b = append(b, `,"id":"`...)
		b = append(b, evt.ID.Hex()...)
		b = append(b, '"')
	}
	if evt.PubKey != ZeroPK {
		b = append(b, `,"pubkey":"`...)
		b = append(b, evt.PubKey.Hex()...)
		b = append(b, '"')
	}
	b = append(b, `,"created_at":`...)
	b = append(b, strconv.FormatInt(int64(evt.CreatedAt), 10)...)
	b = append(b, `,"tags":[`...)
	for i, tag := range evt.Tags {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, '[')
		for j, s := range tag {
			if j > 0 {
				b = append(b, ',')
			}
			b = escapeString(b, s)
		}
		b = append(b, ']')
	}
	b = append(b, `],"content":`...)
	b = escapeString(b, evt.Content)
	if evt.Sig != ([64]byte{}) {
		b = append(b, `,"sig":"`...)
		b = append(b, hex.EncodeToString(evt.Sig[:])...)
		b = append(b, '"')
	}
	b = append(b, '}')

	return b, nil
}

func (evt *Event) UnmarshalJSON(data []byte) error {
	r := gjson.ParseBytes(data)

	evt.Kind = Kind(r.Get("kind").Int())

	idHex := r.Get("id").String()
	if len(idHex) == 64 {
		b, _ := hex.DecodeString(idHex)
		copy(evt.ID[:], b)
	}

	pubkeyHex := r.Get("pubkey").String()
	if len(pubkeyHex) == 64 {
		b, _ := hex.DecodeString(pubkeyHex)
		copy(evt.PubKey[:], b)
	}

	evt.CreatedAt = Timestamp(r.Get("created_at").Int())

	tagsResult := r.Get("tags").Array()
	evt.Tags = make(Tags, len(tagsResult))
	for i, tagResult := range tagsResult {
		tagArr := tagResult.Array()
		evt.Tags[i] = make(Tag, len(tagArr))
		for j, tagItem := range tagArr {
			evt.Tags[i][j] = tagItem.String()
		}
	}

	evt.Content = r.Get("content").String()

	sigHex := r.Get("sig").String()
	if len(sigHex) == 128 {
		b, _ := hex.DecodeString(sigHex)
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
