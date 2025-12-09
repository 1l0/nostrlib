//go:build tinygo

package nostr

import (
	"errors"
	"fmt"
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
		return json.Unmarshal([]byte(arr[1].Raw), &v.Event)
	case 3:
		subid := arr[1].String()
		v.SubscriptionID = &subid
		return json.Unmarshal([]byte(arr[2].Raw), &v.Event)
	default:
		return fmt.Errorf("failed to decode EVENT envelope")
	}
}

func (v EventEnvelope) MarshalJSON() ([]byte, error) {
	// Manual marshaling to match the array structure ["EVENT", ...]
	// We can use a temporary struct or build it manually.
	// Building manually is safer to avoid reflection overhead if possible, but json.Marshal is fine.
	if v.SubscriptionID != nil {
		return json.Marshal([]interface{}{"EVENT", *v.SubscriptionID, v.Event})
	}
	return json.Marshal([]interface{}{"EVENT", v.Event})
}

// ReqEnvelope represents a REQ message.
type ReqEnvelope struct {
	SubscriptionID string
	Filters        []Filter
}

func (_ ReqEnvelope) Label() string { return "REQ" }
func (c ReqEnvelope) String() string {
	v, _ := json.Marshal(c)
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
		if err := json.Unmarshal([]byte(filterj.Raw), &v.Filters[i]); err != nil {
			return fmt.Errorf("on filter: %w", err)
		}
	}

	return nil
}

func (v ReqEnvelope) MarshalJSON() ([]byte, error) {
	data := make([]interface{}, 2+len(v.Filters))
	data[0] = "REQ"
	data[1] = v.SubscriptionID
	for i, f := range v.Filters {
		data[2+i] = f
	}
	return json.Marshal(data)
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
	v, _ := json.Marshal(c)
	return string(v)
}

func (v *CountEnvelope) FromJSON(data string) error {
	r := gjson.Parse(data)
	arr := r.Array()
	if len(arr) < 3 {
		return fmt.Errorf("failed to decode COUNT envelope: missing filters")
	}
	v.SubscriptionID = arr[1].String()

	var countResult struct {
		Count *uint32 `json:"count"`
		HLL   string  `json:"hll"`
	}
	// Try to unmarshal as count result first
	if err := json.Unmarshal([]byte(arr[2].Raw), &countResult); err == nil && countResult.Count != nil {
		v.Count = countResult.Count
		if len(countResult.HLL) > 0 {
			hll, err := HexDecodeString(countResult.HLL)
			if err != nil {
				return fmt.Errorf("invalid \"hll\" value in COUNT message: %w", err)
			}
			v.HyperLogLog = hll
		}
		return nil
	}

	// Otherwise it's a filter
	if err := json.Unmarshal([]byte(arr[2].Raw), &v.Filter); err != nil {
		return fmt.Errorf("on filter: %w", err)
	}

	return nil
}

func (v CountEnvelope) MarshalJSON() ([]byte, error) {
	if v.Count != nil {
		res := struct {
			Count *uint32 `json:"count"`
			HLL   string  `json:"hll,omitempty"`
		}{
			Count: v.Count,
		}
		if v.HyperLogLog != nil {
			res.HLL = HexEncodeToString(v.HyperLogLog)
		}
		return json.Marshal([]interface{}{"COUNT", v.SubscriptionID, res})
	}
	return json.Marshal([]interface{}{"COUNT", v.SubscriptionID, v.Filter})
}

// NoticeEnvelope represents a NOTICE message.
type NoticeEnvelope string

func (_ NoticeEnvelope) Label() string { return "NOTICE" }
func (n NoticeEnvelope) String() string {
	v, _ := json.Marshal(n)
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
	return json.Marshal([]interface{}{"NOTICE", string(v)})
}

// EOSEEnvelope represents an EOSE (End of Stored Events) message.
type EOSEEnvelope string

func (_ EOSEEnvelope) Label() string { return "EOSE" }
func (e EOSEEnvelope) String() string {
	v, _ := json.Marshal(e)
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
	return json.Marshal([]interface{}{"EOSE", string(v)})
}

// CloseEnvelope represents a CLOSE message.
type CloseEnvelope string

func (_ CloseEnvelope) Label() string { return "CLOSE" }
func (c CloseEnvelope) String() string {
	v, _ := json.Marshal(c)
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
	return json.Marshal([]interface{}{"CLOSE", string(v)})
}

// ClosedEnvelope represents a CLOSED message.
type ClosedEnvelope struct {
	SubscriptionID string
	Reason         string
}

func (_ ClosedEnvelope) Label() string { return "CLOSED" }
func (c ClosedEnvelope) String() string {
	v, _ := json.Marshal(c)
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
	return json.Marshal([]interface{}{"CLOSED", v.SubscriptionID, v.Reason})
}

// OKEnvelope represents an OK message.
type OKEnvelope struct {
	EventID ID
	OK      bool
	Reason  string
}

func (_ OKEnvelope) Label() string { return "OK" }
func (o OKEnvelope) String() string {
	v, _ := json.Marshal(o)
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
	return json.Marshal([]interface{}{"OK", HexEncodeToString(v.EventID[:]), v.OK, v.Reason})
}

// AuthEnvelope represents an AUTH message.
type AuthEnvelope struct {
	Challenge *string
	Event     Event
}

func (_ AuthEnvelope) Label() string { return "AUTH" }
func (a AuthEnvelope) String() string {
	v, _ := json.Marshal(a)
	return string(v)
}

func (v *AuthEnvelope) FromJSON(data string) error {
	r := gjson.Parse(data)
	arr := r.Array()
	if len(arr) < 2 {
		return fmt.Errorf("failed to decode Auth envelope: missing fields")
	}
	if arr[1].IsObject() {
		return json.Unmarshal([]byte(arr[1].Raw), &v.Event)
	} else {
		challenge := arr[1].String()
		v.Challenge = &challenge
	}
	return nil
}

func (v AuthEnvelope) MarshalJSON() ([]byte, error) {
	if v.Challenge != nil {
		return json.Marshal([]interface{}{"AUTH", *v.Challenge})
	}
	return json.Marshal([]interface{}{"AUTH", v.Event})
}
