package nip05

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"fiatjaf.com/nostr"
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
