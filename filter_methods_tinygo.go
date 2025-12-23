//go:build tinygo

package nostr

import (
	"encoding/hex"
	stdjson "encoding/json"
	"strconv"
)

func (ef Filter) String() string {
	j, _ := json.Marshal(ef)
	return string(j)
}

func (ef Filter) MarshalJSON() ([]byte, error) {
	var b []byte
	b = append(b, '{')

	first := true

	if len(ef.IDs) > 0 {
		if !first {
			b = append(b, ',')
		}
		first = false
		b = append(b, `"ids":[`...)
		for i, id := range ef.IDs {
			if i > 0 {
				b = append(b, ',')
			}
			b = append(b, '"')
			b = append(b, id.Hex()...)
			b = append(b, '"')
		}
		b = append(b, ']')
	}

	if len(ef.Kinds) > 0 {
		if !first {
			b = append(b, ',')
		}
		first = false
		b = append(b, `"kinds":[`...)
		for i, kind := range ef.Kinds {
			if i > 0 {
				b = append(b, ',')
			}
			b = append(b, strconv.Itoa(int(kind))...)
		}
		b = append(b, ']')
	}

	if len(ef.Authors) > 0 {
		if !first {
			b = append(b, ',')
		}
		first = false
		b = append(b, `"authors":[`...)
		for i, pk := range ef.Authors {
			if i > 0 {
				b = append(b, ',')
			}
			b = append(b, '"')
			b = append(b, pk.Hex()...)
			b = append(b, '"')
		}
		b = append(b, ']')
	}

	if ef.Since != 0 {
		if !first {
			b = append(b, ',')
		}
		first = false
		b = append(b, `"since":`...)
		b = append(b, strconv.FormatInt(int64(ef.Since), 10)...)
	}

	if ef.Until != 0 {
		if !first {
			b = append(b, ',')
		}
		first = false
		b = append(b, `"until":`...)
		b = append(b, strconv.FormatInt(int64(ef.Until), 10)...)
	}

	if ef.Limit > 0 || ef.LimitZero {
		if !first {
			b = append(b, ',')
		}
		first = false
		b = append(b, `"limit":`...)
		b = append(b, strconv.Itoa(ef.Limit)...)
	}

	if ef.Search != "" {
		if !first {
			b = append(b, ',')
		}
		first = false
		b = append(b, `"search":`...)
		sb, _ := json.Marshal(ef.Search)
		b = append(b, sb...)
	}

	for key, values := range ef.Tags {
		if !first {
			b = append(b, ',')
		}
		first = false
		b = append(b, `"#`...)
		b = append(b, key...)
		b = append(b, `":`...)
		vb, _ := json.Marshal(values)
		b = append(b, vb...)
	}

	b = append(b, '}')
	return b, nil
}

func (ef *Filter) UnmarshalJSON(data []byte) error {
	var raw map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(data, &raw); err != nil {
		return err
	}

	for key, val := range raw {
		switch key {
		case "ids":
			var ids []string
			if err := stdjson.Unmarshal(val, &ids); err != nil {
				return err
			}
			for _, s := range ids {
				var id ID
				if len(s) == 64 {
					b, _ := hex.DecodeString(s)
					copy(id[:], b)
				}
				ef.IDs = append(ef.IDs, id)
			}
		case "kinds":
			if err := stdjson.Unmarshal(val, &ef.Kinds); err != nil {
				return err
			}
		case "authors":
			var authors []string
			if err := stdjson.Unmarshal(val, &authors); err != nil {
				return err
			}
			for _, s := range authors {
				var pk PubKey
				if len(s) == 64 {
					b, _ := hex.DecodeString(s)
					copy(pk[:], b)
				}
				ef.Authors = append(ef.Authors, pk)
			}
		case "since":
			if err := stdjson.Unmarshal(val, &ef.Since); err != nil {
				return err
			}
		case "until":
			if err := stdjson.Unmarshal(val, &ef.Until); err != nil {
				return err
			}
		case "limit":
			if err := stdjson.Unmarshal(val, &ef.Limit); err != nil {
				return err
			}
			if ef.Limit == 0 {
				ef.LimitZero = true
			}
		case "search":
			if err := stdjson.Unmarshal(val, &ef.Search); err != nil {
				return err
			}
		default:
			if len(key) > 1 && key[0] == '#' {
				if ef.Tags == nil {
					ef.Tags = make(TagMap)
				}
				var values []string
				if err := stdjson.Unmarshal(val, &values); err != nil {
					return err
				}
				ef.Tags[key[1:]] = values
			}
		}
	}
	return nil
}
