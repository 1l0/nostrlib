package schema

import (
	"testing"

	"fiatjaf.com/nostr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewValidatorFromURL(t *testing.T) {
	v, err := NewValidatorFromURL(DefaultSchemaURL)
	require.NoError(t, err)
	require.NotNil(t, v.Schema)
	require.False(t, v.FailOnUnknown) // default value

	// test with some known kinds from schema.yaml
	_, hasKind0 := v.Schema["0"] // profile metadata
	require.True(t, hasKind0)
	_, hasKind1 := v.Schema["1"] // text note
	require.True(t, hasKind1)
	_, hasKind1111 := v.Schema["1111"] // comment
	require.True(t, hasKind1111)
}

func TestNewValidatorFromBytesWithInvalidYAML(t *testing.T) {
	_, err := NewValidatorFromBytes([]byte("invalid yaml content: [[["))
	require.Error(t, err)
}

func TestValidateEvent_BasicSuccess(t *testing.T) {
	v, err := NewValidatorFromURL(DefaultSchemaURL)
	require.NoError(t, err)

	// kind 1 with free content and valid p tag
	evt := nostr.Event{
		Kind:    1,
		Content: "hello world",
		Tags: nostr.Tags{
			nostr.Tag{"p", "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"},
		},
	}

	err = v.ValidateEvent(evt)
	require.NoError(t, err)
}

func TestValidateEvent_Kind0_JSONContent(t *testing.T) {
	v, err := NewValidatorFromURL(DefaultSchemaURL)
	require.NoError(t, err)

	// valid JSON content
	evt := nostr.Event{
		Kind:    0,
		Content: `{"name":"test","about":"description"}`,
	}
	err = v.ValidateEvent(evt)
	require.NoError(t, err)

	// invalid JSON content
	evt.Content = "not-json-content"
	err = v.ValidateEvent(evt)
	require.Error(t, err)
	require.Equal(t, ContentError{Err: ErrInvalidJson}, err)
}

func TestValidateEvent_UnknownKind(t *testing.T) {
	v, err := NewValidatorFromURL(DefaultSchemaURL)
	require.NoError(t, err)

	evt := nostr.Event{
		Kind:    nostr.Kind(39999),
		Content: "test",
	}

	// should not fail when FailOnUnknown is false (default)
	err = v.ValidateEvent(evt)
	require.NoError(t, err)

	// should fail when FailOnUnknown is true
	v.FailOnUnknown = true
	err = v.ValidateEvent(evt)
	require.Error(t, err)
	require.Equal(t, ErrUnknownKind, err)
}

func TestValidateEvent_EmptyTag(t *testing.T) {
	v, err := NewValidatorFromURL(DefaultSchemaURL)
	require.NoError(t, err)

	evt := nostr.Event{
		Kind:    1,
		Content: "test",
		Tags: nostr.Tags{
			nostr.Tag{}, // empty tag
		},
	}

	err = v.ValidateEvent(evt)
	require.Error(t, err)
	require.Equal(t, ErrEmptyTag, err)
}

func TestValidateNext_ID(t *testing.T) {
	v, err := NewValidatorFromURL(DefaultSchemaURL)
	require.NoError(t, err)

	tests := []struct {
		name  string
		tag   nostr.Tag
		valid bool
	}{
		{
			name:  "valid id",
			tag:   nostr.Tag{"e", "dc90c95f09947507c1044e8f48bcf6350aa6bff1507dd4acfc755b9239b5c962"},
			valid: true,
		},
		{
			name:  "invalid id - too short",
			tag:   nostr.Tag{"e", "dc90c95f09947507c1044e8f48bcf6350aa6bff1507dd4acfc755b9239b5c9"},
			valid: false,
		},
		{
			name:  "invalid id - not hex",
			tag:   nostr.Tag{"e", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := &nextSpec{Type: "id", Required: true}
			_, err := v.validateNext(tt.tag, 1, next)
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestValidateNext_PubKey(t *testing.T) {
	v, err := NewValidatorFromURL(DefaultSchemaURL)
	require.NoError(t, err)

	tests := []struct {
		name  string
		tag   nostr.Tag
		valid bool
	}{
		{
			name:  "valid pubkey",
			tag:   nostr.Tag{"p", "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"},
			valid: true,
		},
		{
			name:  "invalid pubkey - too short",
			tag:   nostr.Tag{"p", "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa45"},
			valid: false,
		},
		{
			name:  "invalid pubkey - not hex",
			tag:   nostr.Tag{"p", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := &nextSpec{Type: "pubkey", Required: true}
			_, err := v.validateNext(tt.tag, 1, next)
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestValidateNext_Relay(t *testing.T) {
	v, err := NewValidatorFromURL(DefaultSchemaURL)
	require.NoError(t, err)

	tests := []struct {
		name  string
		tag   nostr.Tag
		valid bool
	}{
		{
			name:  "valid wss relay",
			tag:   nostr.Tag{"r", "wss://relay.example.com"},
			valid: true,
		},
		{
			name:  "valid ws relay",
			tag:   nostr.Tag{"r", "ws://relay.example.com"},
			valid: true,
		},
		{
			name:  "invalid relay - http",
			tag:   nostr.Tag{"r", "http://relay.example.com"},
			valid: false,
		},
		{
			name:  "invalid relay - https",
			tag:   nostr.Tag{"r", "https://relay.example.com"},
			valid: false,
		},
		{
			name:  "invalid relay - malformed url",
			tag:   nostr.Tag{"r", "not-a-url"},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := &nextSpec{Type: "relay", Required: true}
			_, err := v.validateNext(tt.tag, 1, next)
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestValidateNext_Kind(t *testing.T) {
	v, err := NewValidatorFromURL(DefaultSchemaURL)
	require.NoError(t, err)

	tests := []struct {
		name  string
		tag   nostr.Tag
		valid bool
	}{
		{
			name:  "valid kind",
			tag:   nostr.Tag{"k", "1"},
			valid: true,
		},
		{
			name:  "valid kind - large number",
			tag:   nostr.Tag{"k", "30023"},
			valid: true,
		},
		{
			name:  "invalid kind - not number",
			tag:   nostr.Tag{"k", "not-a-number"},
			valid: false,
		},
		{
			name:  "invalid kind - negative",
			tag:   nostr.Tag{"k", "-1"},
			valid: false,
		},
		{
			name:  "invalid kind - too large",
			tag:   nostr.Tag{"k", "99999"},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := &nextSpec{Type: "kind", Required: true}
			_, err := v.validateNext(tt.tag, 1, next)
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestValidateNext_Constrained(t *testing.T) {
	v, err := NewValidatorFromURL(DefaultSchemaURL)
	require.NoError(t, err)

	tests := []struct {
		name    string
		tag     nostr.Tag
		allowed []string
		valid   bool
	}{
		{
			name:    "valid constrained value",
			tag:     nostr.Tag{"e", "someid", "somerelay", "reply"},
			allowed: []string{"reply", "root"},
			valid:   true,
		},
		{
			name:    "invalid constrained value",
			tag:     nostr.Tag{"e", "someid", "somerelay", "invalid"},
			allowed: []string{"reply", "root"},
			valid:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := &nextSpec{Type: "constrained", Required: true, Either: tt.allowed}
			_, err := v.validateNext(tt.tag, 3, next)
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestValidateNext_GitCommit(t *testing.T) {
	v, err := NewValidatorFromURL(DefaultSchemaURL)
	require.NoError(t, err)

	tests := []struct {
		name  string
		tag   nostr.Tag
		valid bool
	}{
		{
			name:  "valid git commit",
			tag:   nostr.Tag{"r", "a1b2c3d4e5f6789012345678901234567890abcd"},
			valid: true,
		},
		{
			name:  "invalid git commit - too short",
			tag:   nostr.Tag{"r", "a1b2c3d4e5f6789012345678901234567890abc"},
			valid: false,
		},
		{
			name:  "invalid git commit - too long",
			tag:   nostr.Tag{"r", "a1b2c3d4e5f6789012345678901234567890abcde"},
			valid: false,
		},
		{
			name:  "invalid git commit - not hex",
			tag:   nostr.Tag{"r", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := &nextSpec{Type: "gitcommit", Required: true}
			_, err := v.validateNext(tt.tag, 1, next)
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestValidateNext_Addr(t *testing.T) {
	v, err := NewValidatorFromURL(DefaultSchemaURL)
	require.NoError(t, err)

	tests := []struct {
		name  string
		tag   nostr.Tag
		valid bool
	}{
		{
			name:  "valid addr",
			tag:   nostr.Tag{"a", "30023:3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d:test"},
			valid: true,
		},
		{
			name:  "invalid addr - malformed",
			tag:   nostr.Tag{"a", "invalid-addr"},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := &nextSpec{Type: "addr", Required: true}
			_, err := v.validateNext(tt.tag, 1, next)
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestValidateNext_Free(t *testing.T) {
	v, err := NewValidatorFromURL(DefaultSchemaURL)
	require.NoError(t, err)

	// free type should accept anything
	tag := nostr.Tag{"test", "any value here", "even", "multiple", "values"}
	next := &nextSpec{Type: "free", Required: true}
	_, err = v.validateNext(tag, 1, next)
	require.NoError(t, err)
}

func TestValidateNext_UnknownType(t *testing.T) {
	v, err := NewValidatorFromURL(DefaultSchemaURL)
	require.NoError(t, err)

	tag := nostr.Tag{"test", "value"}
	next := &nextSpec{Type: "unknown-type", Required: true}

	// should not fail when FailOnUnknown is false (default)
	_, err = v.validateNext(tag, 1, next)
	require.NoError(t, err)

	// should fail when FailOnUnknown is true
	v.FailOnUnknown = true
	_, err = v.validateNext(tag, 1, next)
	require.Error(t, err)
	require.Equal(t, ErrUnknownTagType, err)
}

func TestValidateNext_RequiredField(t *testing.T) {
	v, err := NewValidatorFromURL(DefaultSchemaURL)
	require.NoError(t, err)

	// test missing required field
	tag := nostr.Tag{"test"} // only name, missing required value
	next := &nextSpec{Type: "free", Required: true}
	_, err = v.validateNext(tag, 1, next)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing index 1")

	// test optional field
	next = &nextSpec{Type: "free", Required: false}
	_, err = v.validateNext(tag, 1, next)
	require.NoError(t, err)
}

func TestValidateNext_Variadic(t *testing.T) {
	v, err := NewValidatorFromURL(DefaultSchemaURL)
	require.NoError(t, err)

	// test variadic field with multiple values
	tag := nostr.Tag{"test", "value1", "value2", "value3"}
	next := &nextSpec{Type: "free", Variadic: true}
	_, err = v.validateNext(tag, 1, next)
	require.NoError(t, err)

	// test variadic field with single value
	tag = nostr.Tag{"test", "only-one-value"}
	_, err = v.validateNext(tag, 1, next)
	require.NoError(t, err)

	// test variadic field with no values (should fail if required)
	tag = nostr.Tag{"test"}
	next = &nextSpec{Type: "free", Variadic: true, Required: true}
	_, err = v.validateNext(tag, 1, next)
	require.Error(t, err)
}

func TestValidateEvent_Kind10002(t *testing.T) {
	v, err := NewValidatorFromURL(DefaultSchemaURL)
	require.NoError(t, err)

	// kind 10002 (relay list metadata) with valid r tags
	evt := nostr.Event{
		Kind:    10002,
		Content: "", // should be empty
		Tags: nostr.Tags{
			nostr.Tag{"r", "wss://relay1.example.com", "read"},
			nostr.Tag{"r", "wss://relay2.example.com", "write"},
		},
	}

	err = v.ValidateEvent(evt)
	require.NoError(t, err)

	// test with invalid relay marker
	evt.Tags = nostr.Tags{
		nostr.Tag{"r", "wss://relay1.example.com", "invalid"},
	}
	err = v.ValidateEvent(evt)
	require.Error(t, err)

	// test with missing required r tags
	evt.Tags = nostr.Tags{} // no r tags at all
	err = v.ValidateEvent(evt)
	require.NoError(t, err) // should pass as tags are not required by schema
}

func TestValidateEvent_Kind1_ETag(t *testing.T) {
	v, err := NewValidatorFromURL(DefaultSchemaURL)
	require.NoError(t, err)

	// kind 1 with e tag (reply/root marker)
	evt := nostr.Event{
		Kind:    1,
		Content: "this is a reply",
		Tags: nostr.Tags{
			nostr.Tag{"e", "dc90c95f09947507c1044e8f48bcf6350aa6bff1507dd4acfc755b9239b5c962", "wss://relay.example.com", "reply", "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"},
		},
	}

	err = v.ValidateEvent(evt)
	require.NoError(t, err)

	// test with invalid marker
	evt.Tags = nostr.Tags{
		nostr.Tag{"e", "dc90c95f09947507c1044e8f48bcf6350aa6bff1507dd4acfc755b9239b5c962", "wss://relay.example.com", "invalid"},
	}
	err = v.ValidateEvent(evt)
	require.Error(t, err)
}

func TestValidateEvent_Kind30617_RepositoryAnnouncement(t *testing.T) {
	v, err := NewValidatorFromURL(DefaultSchemaURL)
	require.NoError(t, err)

	// kind 30617 (repository announcement) with required tags
	evt := nostr.Event{
		Kind:    30617,
		Content: "", // should be empty
		Tags: nostr.Tags{
			nostr.Tag{"d", "my-repo"},
			nostr.Tag{"name", "My Repository"},
			nostr.Tag{"description", "A test repository"},
			nostr.Tag{"web", "https://github.com/user/repo"},
			nostr.Tag{"clone", "https://github.com/user/repo.git"},
			nostr.Tag{"relays", "wss://relay1.example.com", "wss://relay2.example.com"},
			nostr.Tag{"r", "a1b2c3d4e5f6789012345678901234567890abcd", "euc"},
			nostr.Tag{"maintainers", "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"},
		},
	}

	err = v.ValidateEvent(evt)
	require.NoError(t, err)
}

func TestValidateEvent_MultiplePossibleTagSpecs(t *testing.T) {
	v, err := NewValidatorFromURL(DefaultSchemaURL)
	require.NoError(t, err)

	// test event with tags that could match multiple specs
	// kind 1 has both "e" and "q" tags that can take different forms
	evt := nostr.Event{
		Kind:    1,
		Content: "test content",
		Tags: nostr.Tags{
			// this should match the q tag with addr format
			nostr.Tag{"q", "30023:3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d:test", "wss://relay.example.com"},
		},
	}

	err = v.ValidateEvent(evt)
	require.NoError(t, err)

	// test q tag with id format (alternative spec)
	evt.Tags = nostr.Tags{
		nostr.Tag{"q", "dc90c95f09947507c1044e8f48bcf6350aa6bff1507dd4acfc755b9239b5c962", "wss://relay.example.com", "3bf0c63fcb93463407af97a5e5ee64fa883d107ef9e558472c4eb9aaaefa459d"},
	}
	err = v.ValidateEvent(evt)
	require.NoError(t, err)
}

func TestSchema_ErrorMessages(t *testing.T) {
	v, err := NewValidatorFromURL(DefaultSchemaURL)
	require.NoError(t, err)

	tests := []struct {
		name     string
		event    nostr.Event
		expError error
	}{
		{
			name: "empty tag error",
			event: nostr.Event{
				Kind:    1,
				Content: "test",
				Tags:    nostr.Tags{nostr.Tag{}},
			},
			expError: ErrEmptyTag,
		},
		{
			name: "unknown kind error when FailOnUnknown is true",
			event: nostr.Event{
				Kind: nostr.Kind(39999),
			},
			expError: ErrUnknownKind,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expError == ErrUnknownKind {
				v.FailOnUnknown = true
				defer func() { v.FailOnUnknown = false }()
			}

			err := v.ValidateEvent(tt.event)
			require.Error(t, err)
			assert.Equal(t, tt.expError, err)
		})
	}
}
