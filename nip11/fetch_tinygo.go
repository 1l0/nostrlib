//go:build tinygo

package nip11

import (
	"context"
	"fmt"
	"time"

	"fiatjaf.com/nostr"
	fetch "marwan.io/wasm-fetch"
)

// Fetch fetches the NIP-11 metadata for a relay.
//
// It will always return `info` with at least `URL` filled -- even if we can't connect to the
// relay or if it doesn't have a NIP-11 handler -- although in that case it will also return
// an error.
func Fetch(ctx context.Context, u string) (info RelayInformationDocument, err error) {
	// normalize URL to start with http://, https:// or without protocol
	u = nostr.NormalizeURL(u)
	if len(u) < 8 {
		return info, fmt.Errorf("invalid url %s", u)
	}

	info = RelayInformationDocument{
		URL: u,
	}

	if _, ok := ctx.Deadline(); !ok {
		// if no timeout is set, force it to 7 seconds
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 7*time.Second)
		defer cancel()
	}

	// make request
	headers := make(map[string]string, 2)
	headers["Accept"] = "application/nostr+json"
	headers["User-Agent"] = "nostrlib/go"
	r, err := fetch.Fetch("http"+u[2:], &fetch.Opts{
		Method:  fetch.MethodGet,
		Signal:  ctx,
		Headers: headers,
	})
	if err != nil {
		return info, fmt.Errorf("fetch failed: %w", err)
	}
	if r.Status != 200 && r.Status != 204 {
		return info, fmt.Errorf("fetch status is not ok: %d", r.Status)
	}
	if err := json.Unmarshal(r.Body, &info); err != nil {
		return info, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return info, nil
}
