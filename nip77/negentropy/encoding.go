package negentropy

import (
	"bytes"
	"fmt"

	"fiatjaf.com/nostr"
)

func (n *Negentropy) readTimestamp(reader *bytes.Reader) (nostr.Timestamp, error) {
	delta, err := readVarInt(reader)
	if err != nil {
		return 0, err
	}

	if delta == 0 {
		// zeroes are infinite
		timestamp := maxTimestamp
		n.lastTimestampIn = timestamp
		return timestamp, nil
	}

	// remove 1 as we always add 1 when encoding
	delta--

	// we add the previously cached timestamp to get the current
	timestamp := n.lastTimestampIn + nostr.Timestamp(delta)

	// cache this so we can apply it to the delta next time
	n.lastTimestampIn = timestamp

	return timestamp, nil
}

func (n *Negentropy) readBound(reader *bytes.Reader) (Bound, error) {
	timestamp, err := n.readTimestamp(reader)
	if err != nil {
		return Bound{}, fmt.Errorf("failed to decode bound timestamp: %w", err)
	}

	length, err := readVarInt(reader)
	if err != nil {
		return Bound{}, fmt.Errorf("failed to decode bound length: %w", err)
	}

	pfb := make([]byte, length)
	if _, err := reader.Read(pfb); err != nil {
		return Bound{}, fmt.Errorf("failed to read bound id: %w", err)
	}

	return Bound{timestamp, pfb}, nil
}

func (n *Negentropy) writeTimestamp(w *bytes.Buffer, timestamp nostr.Timestamp) {
	if timestamp == maxTimestamp {
		// zeroes are infinite
		n.lastTimestampOut = maxTimestamp // cache this (see below)
		writeVarInt(w, 0)
		return
	}

	// we will only encode the difference between this timestamp and the previous
	delta := timestamp - n.lastTimestampOut

	// we cache this here as the next timestamp we encode will be just a delta from this
	n.lastTimestampOut = timestamp

	// add 1 to prevent zeroes from being read as infinites
	writeVarInt(w, int(delta+1))
	return
}

func (n *Negentropy) writeBound(w *bytes.Buffer, bound Bound) {
	n.writeTimestamp(w, bound.Timestamp)
	writeVarInt(w, len(bound.IDPrefix))
	w.Write(bound.IDPrefix)
}

func getMinimalBound(prev, curr Item) Bound {
	if curr.Timestamp != prev.Timestamp {
		return Bound{curr.Timestamp, nil}
	}

	sharedPrefixBytes := 0
	for i := 0; i < 31; i++ {
		if curr.ID[i] != prev.ID[i] {
			break
		}
		sharedPrefixBytes++
	}

	// sharedPrefixBytes + 1 to include the first differing byte, or the entire ID if identical.
	return Bound{curr.Timestamp, curr.ID[:(sharedPrefixBytes + 1)]}
}

func readVarInt(reader *bytes.Reader) (int, error) {
	var res int = 0

	for {
		b, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}

		res = (res << 7) | (int(b) & 127)
		if (b & 128) == 0 {
			break
		}
	}

	return res, nil
}

func writeVarInt(w *bytes.Buffer, n int) {
	if n == 0 {
		w.WriteByte(0)
		return
	}

	w.Write(EncodeVarInt(n))
}

func EncodeVarInt(n int) []byte {
	if n == 0 {
		return []byte{0}
	}

	result := make([]byte, 8)
	idx := 7

	for n != 0 {
		result[idx] = byte(n & 0x7F)
		n >>= 7
		idx--
	}

	result = result[idx+1:]
	for i := 0; i < len(result)-1; i++ {
		result[i] |= 0x80
	}

	return result
}
