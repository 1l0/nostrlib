//go:build tinygo

package nostr

import (
	"encoding/hex"
	stdjson "encoding/json"
	"strconv"

	"github.com/tidwall/gjson"
)

func (ef Filter) String() string {
	j, _ := stdjson.Marshal(ef)
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
		b = escapeString(b, ef.Search)
	}

	for key, values := range ef.Tags {
		if !first {
			b = append(b, ',')
		}
		first = false
		b = append(b, `"#`...)
		b = append(b, key...)
		b = append(b, `":[`...)
		for i, v := range values {
			if i > 0 {
				b = append(b, ',')
			}
			b = escapeString(b, v)
		}
		b = append(b, ']')
	}

	b = append(b, '}')
	return b, nil
}

func (ef *Filter) UnmarshalJSON(data []byte) error {
	r := gjson.ParseBytes(data)

	r.ForEach(func(key, val gjson.Result) bool {
		switch key.String() {
		case "ids":
			idsResult := val.Array()
			for _, s := range idsResult {
				var id ID
				hexStr := s.String()
				if len(hexStr) == 64 {
					b, _ := hex.DecodeString(hexStr)
					copy(id[:], b)
				}
				ef.IDs = append(ef.IDs, id)
			}
		case "kinds":
			kindsResult := val.Array()
			for _, k := range kindsResult {
				ef.Kinds = append(ef.Kinds, Kind(k.Int()))
			}
		case "authors":
			authorsResult := val.Array()
			for _, s := range authorsResult {
				var pk PubKey
				hexStr := s.String()
				if len(hexStr) == 64 {
					b, _ := hex.DecodeString(hexStr)
					copy(pk[:], b)
				}
				ef.Authors = append(ef.Authors, pk)
			}
		case "since":
			ef.Since = Timestamp(val.Int())
		case "until":
			ef.Until = Timestamp(val.Int())
		case "limit":
			ef.Limit = int(val.Int())
			if ef.Limit == 0 {
				ef.LimitZero = true
			}
		case "search":
			ef.Search = val.String()
		default:
			k := key.String()
			if len(k) > 1 && k[0] == '#' {
				if ef.Tags == nil {
					ef.Tags = make(TagMap)
				}
				valuesResult := val.Array()
				values := make([]string, len(valuesResult))
				for i, v := range valuesResult {
					values[i] = v.String()
				}
				ef.Tags[k[1:]] = values
			}
		}
		return true
	})

	return nil
}
