//go:build !tinygo

package nip05

import (
	"context"
	json "encoding/json"
	"fmt"
	"net/http"
)

func Fetch(ctx context.Context, fullname string) (resp WellKnownResponse, name string, err error) {
	name, domain, err := ParseIdentifier(fullname)
	if err != nil {
		return resp, name, fmt.Errorf("failed to parse '%s': %w", fullname, err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("https://%s/.well-known/nostr.json?name=%s", domain, name), nil)
	if err != nil {
		return resp, name, fmt.Errorf("failed to create a request: %w", err)
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return resp, name, fmt.Errorf("request failed: %w", err)
	}
	defer res.Body.Close()

	var result WellKnownResponse
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return resp, name, fmt.Errorf("failed to decode json response: %w", err)
	}

	return result, name, nil
}
