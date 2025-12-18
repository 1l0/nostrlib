//go:build tinygo

package nip05

import (
	"context"
	json "encoding/json"
	"fmt"

	fetch "marwan.io/wasm-fetch"
)

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
