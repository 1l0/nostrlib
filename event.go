package nostr

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"

	"github.com/mailru/easyjson"
)

// Event represents a Nostr event.
type Event struct {
	ID        ID
	PubKey    PubKey
	CreatedAt Timestamp
	Kind      uint16
	Tags      Tags
	Content   string
	Sig       [64]byte
}

func (evt Event) String() string {
	j, _ := easyjson.Marshal(evt)
	return string(j)
}

// GetID serializes and returns the event ID as a string.
func (evt *Event) GetID() ID {
	return sha256.Sum256(evt.Serialize())
}

// CheckID checks if the implied ID matches the given ID more efficiently.
func (evt *Event) CheckID() bool {
	return evt.GetID() == evt.ID
}

// Serialize outputs a byte array that can be hashed to produce the canonical event "id".
func (evt *Event) Serialize() []byte {
	// the serialization process is just putting everything into a JSON array
	// so the order is kept. See NIP-01
	dst := make([]byte, 0, 100+len(evt.Content)+len(evt.Tags)*80)
	return serializeEventInto(evt, dst)
}

func serializeEventInto(evt *Event, dst []byte) []byte {
	// the header portion is easy to serialize
	// [0,"pubkey",created_at,kind,[
	dst = append(dst, `[0,"`...)
	dst = hex.AppendEncode(dst, evt.PubKey[:])
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
