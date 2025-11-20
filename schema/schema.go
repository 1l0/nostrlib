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

func NewDefaultValidator() Validator {
	return NewValidator(string(schemaFile))
}

var (
	ErrUnknownContent = fmt.Errorf("unknown content")
	ErrUnknownKind    = fmt.Errorf("unknown kind")
	ErrInvalidJson    = fmt.Errorf("invalid json")
	ErrEmptyValue     = fmt.Errorf("can't be empty")
	ErrEmptyTag       = fmt.Errorf("empty tag")
	ErrUnknownTagType = fmt.Errorf("unknown tag type")
	ErrDanglingSpace  = fmt.Errorf("value has dangling space")
)

type ContentError struct {
	Err error
}

func (ce ContentError) Error() string {
	return fmt.Sprintf("content: %s", ce.Err)
}

type TagError struct {
	Tag  int
	Item int
	Err  error
}

func (te TagError) Error() string {
	return fmt.Sprintf("tag[%d][%d]: %s", te.Tag, te.Item, te.Err)
}

func (v *Validator) ValidateEvent(evt nostr.Event) error {
	if !isTrimmed(evt.Content) {
		return ContentError{ErrDanglingSpace}
	}

	if sch, ok := v.Schema[strconv.FormatUint(uint64(evt.Kind), 10)]; ok {
		switch sch.Content {
		case "json":
			if !json.Valid(unsafe.Slice(unsafe.StringData(evt.Content), len(evt.Content))) {
				return ContentError{ErrInvalidJson}
			}
		case "free":
			if evt.Content == "" {
				return ContentError{ErrEmptyValue}
			}
		default:
			if v.FailOnUnknown {
				return ContentError{ErrUnknownContent}
			}
		}

	tags:
		for ti, tag := range evt.Tags {
			if len(tag) == 0 {
				return ErrEmptyTag
			}

			var lastErr error
		specs:
			for _, tagspec := range sch.Tags {
				if tagspec.Name == tag[0] || (tagspec.Prefix != "" && strings.HasPrefix(tag[0], tagspec.Prefix)) {
					if tagspec.Next != nil {
						if ii, err := v.validateNext(tag, 1, tagspec.Next); err != nil {
							lastErr = TagError{ti, ii, err}
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

func (v *Validator) validateNext(tag nostr.Tag, index int, this *nextSpec) (failedIndex int, err error) {
	if len(tag) <= index {
		if this.Required {
			return index, fmt.Errorf("invalid tag '%s', missing index %d", tag[0], index)
		}
		return -1, nil
	}

	if !isTrimmed(tag[index]) {
		return index, ErrDanglingSpace
	}

	switch this.Type {
	case "id":
		if _, err := nostr.IDFromHex(tag[index]); err != nil {
			return index, fmt.Errorf("invalid id at tag '%s', index %d", tag[0], index)
		}
	case "pubkey":
		if _, err := nostr.PubKeyFromHex(tag[index]); err != nil {
			return index, fmt.Errorf("invalid pubkey at tag '%s', index %d", tag[0], index)
		}
	case "addr":
		if _, err := nostr.ParseAddrString(tag[index]); err != nil {
			return index, fmt.Errorf("invalid addr at tag '%s', index %d", tag[0], index)
		}
	case "kind":
		if _, err := strconv.ParseUint(tag[index], 10, 16); err != nil {
			return index, fmt.Errorf("invalid kind at tag '%s', index %d", tag[0], index)
		}
	case "relay":
		if url, err := url.Parse(tag[index]); err != nil || (url.Scheme != "ws" && url.Scheme != "wss") {
			return index, fmt.Errorf("invalid relay at tag '%s', index %d", tag[0], index)
		}
	case "constrained":
		if !slices.Contains(this.Either, tag[index]) {
			return index, fmt.Errorf("invalid constrained at tag '%s', index %d", tag[0], index)
		}
	case "gitcommit":
		if len(tag[index]) != 40 {
			return index, fmt.Errorf("invalid gitcommit at tag '%s', index %d", tag[0], index)
		}
		if _, err := hex.Decode(gitcommitdummydecoder, unsafe.Slice(unsafe.StringData(tag[index]), 40)); err != nil {
			return index, fmt.Errorf("invalid gitcommit at tag '%s', index %d", tag[0], index)
		}
	case "free":
	default:
		if v.FailOnUnknown {
			return index, ErrUnknownTagType
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

	return -1, nil
}
