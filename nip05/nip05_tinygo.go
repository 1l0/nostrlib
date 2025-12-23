//go:build tinygo

package nip05

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"fiatjaf.com/nostr"
	fetch "marwan.io/wasm-fetch"
)

var NIP05_REGEX = regexp.MustCompile(`^(?:([\w.+-]+)@)?([\w_-]+(\.[\w_-]+)+)$`)

type WellKnownResponse struct {
	Names  map[string]nostr.PubKey   `json:"names"`
	Relays map[nostr.PubKey][]string `json:"relays,omitempty"`
	NIP46  map[nostr.PubKey][]string `json:"nip46,omitempty"`
}

func IsValidIdentifier(input string) bool {
	return NIP05_REGEX.MatchString(input)
}

func ParseIdentifier(fullname string) (name string, domain string, err error) {
	res := NIP05_REGEX.FindStringSubmatch(fullname)
	if len(res) == 0 {
		return "", "", fmt.Errorf("invalid identifier")
	}
	if res[1] == "" {
		res[1] = "_"
	}
	return res[1], res[2], nil
}

func QueryIdentifier(ctx context.Context, fullname string) (*nostr.ProfilePointer, error) {
	result, name, err := Fetch(ctx, fullname)
	if err != nil {
		return nil, err
	}

	pubkey, ok := result.Names[name]
	if !ok {
		return nil, fmt.Errorf("no entry for name '%s'", name)
	}

	relays, _ := result.Relays[pubkey]
	return &nostr.ProfilePointer{
		PublicKey: pubkey,
		Relays:    relays,
	}, nil
}

func Fetch(ctx context.Context, fullname string) (resp WellKnownResponse, name string, err error) {
	name, domain, err := ParseIdentifier(fullname)
	if err != nil {
		return resp, name, fmt.Errorf("failed to parse '%s': %w", fullname, err)
	}

	r, err := fetch.Fetch(fmt.Sprintf("https://%s/.well-known/nostr.json?name=%s", domain, name), &fetch.Opts{
		Method: fetch.MethodGet,
		Signal: ctx,
	})
	if err != nil {
		return resp, name, fmt.Errorf("fetch failed: %w", err)
	}
	if r.Status != 200 && r.Status != 204 {
		return resp, name, fmt.Errorf("fetch status is not ok: %d", r.Status)
	}

	var result WellKnownResponse
	if err := json.Unmarshal(r.Body, &result); err != nil {
		return resp, name, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return result, name, nil
}

func NormalizeIdentifier(fullname string) string {
	if strings.HasPrefix(fullname, "_@") {
		return fullname[2:]
	}

	return fullname
}

func IdentifierToURL(address string) string {
	spl := strings.Split(address, "@")
	if len(spl) == 1 {
		return fmt.Sprintf("https://%s/.well-known/nostr.json?name=_", spl[0])
	}
	return fmt.Sprintf("https://%s/.well-known/nostr.json?name=%s", spl[1], spl[0])
}
