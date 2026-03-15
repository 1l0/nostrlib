//go:build tinygo

package nostr

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

var (
	UnknownLabel        = errors.New("unknown envelope label")
	InvalidJsonEnvelope = errors.New("invalid json envelope")
)

func ParseMessage(message string) (Envelope, error) {
	firstQuote := strings.IndexByte(message, '"')
	if firstQuote == -1 {
		return nil, InvalidJsonEnvelope
	}
	secondQuote := strings.IndexByte(message[firstQuote+1:], '"')
	if secondQuote == -1 {
		return nil, InvalidJsonEnvelope
	}
	label := message[firstQuote+1 : firstQuote+1+secondQuote]

	var v Envelope
	switch label {
	case "EVENT":
		v = &EventEnvelope{}
	case "REQ":
		v = &ReqEnvelope{}
	case "COUNT":
		v = &CountEnvelope{}
	case "NOTICE":
		x := NoticeEnvelope("")
		v = &x
	case "EOSE":
		x := EOSEEnvelope("")
		v = &x
	case "OK":
		v = &OKEnvelope{}
	case "AUTH":
		v = &AuthEnvelope{}
	case "CLOSED":
		v = &ClosedEnvelope{}
	case "CLOSE":
		x := CloseEnvelope("")
		v = &x
	default:
		return nil, UnknownLabel
	}

	if err := v.FromJSON(message); err != nil {
		return nil, err
	}

	return v, nil
}

// Envelope is the interface for all nostr message envelopes.
type Envelope interface {
	Label() string
	FromJSON(string) error
	MarshalJSON() ([]byte, error)
	String() string
}

var (
	_ Envelope = (*EventEnvelope)(nil)
	_ Envelope = (*ReqEnvelope)(nil)
	_ Envelope = (*CountEnvelope)(nil)
	_ Envelope = (*NoticeEnvelope)(nil)
	_ Envelope = (*EOSEEnvelope)(nil)
	_ Envelope = (*CloseEnvelope)(nil)
	_ Envelope = (*OKEnvelope)(nil)
	_ Envelope = (*AuthEnvelope)(nil)
)

// EventEnvelope represents an EVENT message.
type EventEnvelope struct {
	SubscriptionID *string
	Event
}

func (_ EventEnvelope) Label() string { return "EVENT" }

func (v *EventEnvelope) FromJSON(data string) error {
	r := gjson.Parse(data)
	arr := r.Array()
	switch len(arr) {
	case 2:
		return v.Event.UnmarshalJSON([]byte(arr[1].Raw))
	case 3:
		subid := arr[1].String()
		v.SubscriptionID = &subid
		return v.Event.UnmarshalJSON([]byte(arr[2].Raw))
	default:
		return fmt.Errorf("failed to decode EVENT envelope")
	}
}

func (v EventEnvelope) MarshalJSON() ([]byte, error) {
	var b []byte
	b = append(b, `["EVENT",`...)
	if v.SubscriptionID != nil {
		b = escapeString(b, *v.SubscriptionID)
		b = append(b, ',')
	}
	eb, err := v.Event.MarshalJSON()
	if err != nil {
		return nil, err
	}
	b = append(b, eb...)
	b = append(b, ']')
	return b, nil
}

// ReqEnvelope represents a REQ message.
type ReqEnvelope struct {
	SubscriptionID string
	Filters        []Filter
}

func (_ ReqEnvelope) Label() string { return "REQ" }
func (c ReqEnvelope) String() string {
	v, _ := c.MarshalJSON()
	return string(v)
}

func (v *ReqEnvelope) FromJSON(data string) error {
	r := gjson.Parse(data)
	arr := r.Array()
	if len(arr) < 3 {
		return fmt.Errorf("failed to decode REQ envelope: missing filters")
	}
	v.SubscriptionID = arr[1].String()

	v.Filters = make([]Filter, len(arr)-2)
	for i, filterj := range arr[2:] {
		if err := v.Filters[i].UnmarshalJSON([]byte(filterj.Raw)); err != nil {
			return fmt.Errorf("on filter: %w", err)
		}
	}

	return nil
}

func (v ReqEnvelope) MarshalJSON() ([]byte, error) {
	var b []byte
	b = append(b, `["REQ",`...)
	b = escapeString(b, v.SubscriptionID)
	for _, f := range v.Filters {
		b = append(b, ',')
		fb, err := f.MarshalJSON()
		if err != nil {
			return nil, err
		}
		b = append(b, fb...)
	}
	b = append(b, ']')
	return b, nil
}

// CountEnvelope represents a COUNT message.
type CountEnvelope struct {
	SubscriptionID string
	Filter
	Count       *uint32
	HyperLogLog []byte
}

func (_ CountEnvelope) Label() string { return "COUNT" }
func (c CountEnvelope) String() string {
	v, _ := c.MarshalJSON()
	return string(v)
}

func (v *CountEnvelope) FromJSON(data string) error {
	r := gjson.Parse(data)
	arr := r.Array()
	if len(arr) < 3 {
		return fmt.Errorf("failed to decode COUNT envelope: missing filters")
	}
	v.SubscriptionID = arr[1].String()

	// Try to parse as count result first (avoid encoding/json to prevent TinyGo reflect panic)
	obj := gjson.Parse(arr[2].Raw)
	if countVal := obj.Get("count"); countVal.Exists() {
		c := uint32(countVal.Uint())
		v.Count = &c
		if hllHex := obj.Get("hll").String(); len(hllHex) > 0 {
			hll, err := HexDecodeString(hllHex)
			if err != nil {
				return fmt.Errorf("invalid \"hll\" value in COUNT message: %w", err)
			}
			v.HyperLogLog = hll
		}
		return nil
	}

	// Otherwise it's a filter
	if err := v.Filter.UnmarshalJSON([]byte(arr[2].Raw)); err != nil {
		return fmt.Errorf("on filter: %w", err)
	}

	return nil
}

func (v CountEnvelope) MarshalJSON() ([]byte, error) {
	var b []byte
	b = append(b, `["COUNT",`...)
	b = escapeString(b, v.SubscriptionID)
	b = append(b, ',')
	if v.Count != nil {
		b = append(b, `{"count":`...)
		b = append(b, strconv.FormatUint(uint64(*v.Count), 10)...)
		if v.HyperLogLog != nil {
			b = append(b, `,"hll":"`...)
			b = append(b, HexEncodeToString(v.HyperLogLog)...)
			b = append(b, '"')
		}
		b = append(b, '}')
	} else {
		fb, err := v.Filter.MarshalJSON()
		if err != nil {
			return nil, err
		}
		b = append(b, fb...)
	}
	b = append(b, ']')
	return b, nil
}

// NoticeEnvelope represents a NOTICE message.
type NoticeEnvelope string

func (_ NoticeEnvelope) Label() string { return "NOTICE" }
func (n NoticeEnvelope) String() string {
	v, _ := n.MarshalJSON()
	return string(v)
}

func (v *NoticeEnvelope) FromJSON(data string) error {
	r := gjson.Parse(data)
	arr := r.Array()
	if len(arr) < 2 {
		return fmt.Errorf("failed to decode NOTICE envelope")
	}
	*v = NoticeEnvelope(arr[1].String())
	return nil
}

func (v NoticeEnvelope) MarshalJSON() ([]byte, error) {
	var b []byte
	b = append(b, `["NOTICE",`...)
	b = escapeString(b, string(v))
	b = append(b, ']')
	return b, nil
}

// EOSEEnvelope represents an EOSE (End of Stored Events) message.
type EOSEEnvelope string

func (_ EOSEEnvelope) Label() string { return "EOSE" }
func (e EOSEEnvelope) String() string {
	v, _ := e.MarshalJSON()
	return string(v)
}

func (v *EOSEEnvelope) FromJSON(data string) error {
	r := gjson.Parse(data)
	arr := r.Array()
	if len(arr) < 2 {
		return fmt.Errorf("failed to decode EOSE envelope")
	}
	*v = EOSEEnvelope(arr[1].String())
	return nil
}

func (v EOSEEnvelope) MarshalJSON() ([]byte, error) {
	var b []byte
	b = append(b, `["EOSE",`...)
	b = escapeString(b, string(v))
	b = append(b, ']')
	return b, nil
}

// CloseEnvelope represents a CLOSE message.
type CloseEnvelope string

func (_ CloseEnvelope) Label() string { return "CLOSE" }
func (c CloseEnvelope) String() string {
	v, _ := c.MarshalJSON()
	return string(v)
}

func (v *CloseEnvelope) FromJSON(data string) error {
	r := gjson.Parse(data)
	arr := r.Array()
	if len(arr) < 2 {
		return fmt.Errorf("failed to decode CLOSE envelope")
	}
	*v = CloseEnvelope(arr[1].String())
	return nil
}

func (v CloseEnvelope) MarshalJSON() ([]byte, error) {
	var b []byte
	b = append(b, `["CLOSE",`...)
	b = escapeString(b, string(v))
	b = append(b, ']')
	return b, nil
}

// ClosedEnvelope represents a CLOSED message.
type ClosedEnvelope struct {
	SubscriptionID string
	Reason         string
}

func (_ ClosedEnvelope) Label() string { return "CLOSED" }
func (c ClosedEnvelope) String() string {
	v, _ := c.MarshalJSON()
	return string(v)
}

func (v *ClosedEnvelope) FromJSON(data string) error {
	r := gjson.Parse(data)
	arr := r.Array()
	if len(arr) < 3 {
		return fmt.Errorf("failed to decode CLOSED envelope")
	}
	*v = ClosedEnvelope{
		SubscriptionID: arr[1].String(),
		Reason:         arr[2].String(),
	}
	return nil
}

func (v ClosedEnvelope) MarshalJSON() ([]byte, error) {
	var b []byte
	b = append(b, `["CLOSED",`...)
	b = escapeString(b, v.SubscriptionID)
	b = append(b, ',')
	b = escapeString(b, v.Reason)
	b = append(b, ']')
	return b, nil
}

// OKEnvelope represents an OK message.
type OKEnvelope struct {
	EventID ID
	OK      bool
	Reason  string
}

func (_ OKEnvelope) Label() string { return "OK" }
func (o OKEnvelope) String() string {
	v, _ := o.MarshalJSON()
	return string(v)
}

func (v *OKEnvelope) FromJSON(data string) error {
	r := gjson.Parse(data)
	arr := r.Array()
	if len(arr) < 4 {
		return fmt.Errorf("failed to decode OK envelope: missing fields")
	}
	b, err := HexDecodeString(arr[1].String())
	if err != nil {
		return err
	}
	copy(v.EventID[:], b)
	v.OK = arr[2].Bool()
	v.Reason = arr[3].String()

	return nil
}

func (v OKEnvelope) MarshalJSON() ([]byte, error) {
	var b []byte
	b = append(b, `["OK","`...)
	b = append(b, HexEncodeToString(v.EventID[:])...)
	b = append(b, `",`...)
	if v.OK {
		b = append(b, "true"...)
	} else {
		b = append(b, "false"...)
	}
	b = append(b, ',')
	b = escapeString(b, v.Reason)
	b = append(b, ']')
	return b, nil
}

// AuthEnvelope represents an AUTH message.
type AuthEnvelope struct {
	Challenge *string
	Event     Event
}

func (_ AuthEnvelope) Label() string { return "AUTH" }
func (a AuthEnvelope) String() string {
	v, _ := a.MarshalJSON()
	return string(v)
}

func (v *AuthEnvelope) FromJSON(data string) error {
	r := gjson.Parse(data)
	arr := r.Array()
	if len(arr) < 2 {
		return fmt.Errorf("failed to decode Auth envelope: missing fields")
	}
	if arr[1].IsObject() {
		return v.Event.UnmarshalJSON([]byte(arr[1].Raw))
	} else {
		challenge := arr[1].String()
		v.Challenge = &challenge
	}
	return nil
}

func (v AuthEnvelope) MarshalJSON() ([]byte, error) {
	var b []byte
	b = append(b, `["AUTH",`...)
	if v.Challenge != nil {
		b = escapeString(b, *v.Challenge)
	} else {
		eb, err := v.Event.MarshalJSON()
		if err != nil {
			return nil, err
		}
		b = append(b, eb...)
	}
	b = append(b, ']')
	return b, nil
}
