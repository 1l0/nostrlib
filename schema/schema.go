package schema

import (
	_ "embed"
	"encoding/hex"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unsafe"

	"fiatjaf.com/nostr"
	"github.com/segmentio/encoding/json"
	"gopkg.in/yaml.v3"
)

//go:embed schema.yaml
var schemaFile []byte

type Schema map[string]KindSchema

type KindSchema struct {
	Content string    `yaml:"content"`
	Tags    []tagSpec `yaml:"tags"`
}

type tagSpec struct {
	Name   string    `yaml:"name"`
	Prefix string    `yaml:"prefix"`
	Next   *nextSpec `yaml:"next"`
}

type nextSpec struct {
	Type     string    `yaml:"type"`
	Required bool      `yaml:"required"`
	Either   []string  `yaml:"either"`
	Next     *nextSpec `yaml:"next"`
	Variadic bool      `yaml:"variadic"`
}

type Validator struct {
	Schema        Schema
	FailOnUnknown bool
}

func NewValidator(schemaData string) Validator {
	schema := make(Schema)
	if err := yaml.Unmarshal([]byte(schemaData), &schema); err != nil {
		panic(fmt.Errorf("failed to parse schema.yaml: %w", err))
	}

	return Validator{Schema: schema}
}

var (
	ErrUnknownContent     = fmt.Errorf("unknown content")
	ErrUnknownKind        = fmt.Errorf("unknown kind")
	ErrInvalidContentJson = fmt.Errorf("invalid content json")
	ErrEmptyTag           = fmt.Errorf("empty tag")
	ErrUnknownTagType     = fmt.Errorf("unknown tag type")
)

func (v *Validator) ValidateEvent(evt nostr.Event) error {
	if sch, ok := v.Schema[strconv.FormatUint(uint64(evt.Kind), 10)]; ok {
		switch sch.Content {
		case "json":
			if !json.Valid(unsafe.Slice(unsafe.StringData(evt.Content), len(evt.Content))) {
				return ErrInvalidContentJson
			}
		case "free":
		default:
			if v.FailOnUnknown {
				return ErrInvalidContentJson
			}
		}

	tags:
		for _, tag := range evt.Tags {
			if len(tag) == 0 {
				return ErrEmptyTag
			}

			var lastErr error
		specs:
			for _, tagspec := range sch.Tags {
				if tagspec.Name == tag[0] || (tagspec.Prefix != "" && strings.HasPrefix(tag[0], tagspec.Prefix)) {
					if tagspec.Next != nil {
						if err := v.validateNext(tag, 1, tagspec.Next); err != nil {
							lastErr = err
							continue specs // see if there is another tagspec that matches this
						} else {
							continue tags
						}
					}
				} else {
					continue specs
				}
			}

			if lastErr != nil {
				// there was at least one failure for this tag and no further successes
				return lastErr
			}
		}
	}

	if v.FailOnUnknown {
		return ErrUnknownKind
	}

	return nil
}

var gitcommitdummydecoder = make([]byte, 20)

func (v *Validator) validateNext(tag nostr.Tag, index int, this *nextSpec) error {
	if len(tag) <= index {
		if this.Required {
			return fmt.Errorf("invalid tag '%s', missing index %d", tag[0], index)
		}
		return nil
	}
	switch this.Type {
	case "id":
		if _, err := nostr.IDFromHex(tag[index]); err != nil {
			return fmt.Errorf("invalid id at tag '%s', index %d", tag[0], index)
		}
	case "pubkey":
		if _, err := nostr.PubKeyFromHex(tag[index]); err != nil {
			return fmt.Errorf("invalid pubkey at tag '%s', index %d", tag[0], index)
		}
	case "addr":
		if _, err := nostr.ParseAddrString(tag[index]); err != nil {
			return fmt.Errorf("invalid addr at tag '%s', index %d", tag[0], index)
		}
	case "kind":
		if _, err := strconv.ParseUint(tag[index], 10, 16); err != nil {
			return fmt.Errorf("invalid kind at tag '%s', index %d", tag[0], index)
		}
	case "relay":
		if url, err := url.Parse(tag[index]); err != nil || (url.Scheme != "ws" && url.Scheme != "wss") {
			return fmt.Errorf("invalid relay at tag '%s', index %d", tag[0], index)
		}
	case "constrained":
		if !slices.Contains(this.Either, tag[index]) {
			return fmt.Errorf("invalid constrained at tag '%s', index %d", tag[0], index)
		}
	case "gitcommit":
		if len(tag[index]) != 40 {
			return fmt.Errorf("invalid gitcommit at tag '%s', index %d", tag[0], index)
		}
		if _, err := hex.Decode(gitcommitdummydecoder, unsafe.Slice(unsafe.StringData(tag[index]), 40)); err != nil {
			return fmt.Errorf("invalid gitcommit at tag '%s', index %d", tag[0], index)
		}
	case "free":
	default:
		if v.FailOnUnknown {
			return ErrUnknownTagType
		}
	}

	if this.Variadic {
		// apply this same validation to all further items
		if len(tag) >= index {
			return v.validateNext(tag, index+1, this)
		}
	}

	if this.Next != nil {
		return v.validateNext(tag, index+1, this.Next)
	}

	return nil
}
