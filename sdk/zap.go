package sdk

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"fiatjaf.com/nostr"
	"github.com/tidwall/gjson"
)

// FetchZapProvider fetches the zap provider public key for a given user from their profile metadata.
// It uses a cache to avoid repeated fetches. If no zap provider is set in the profile, returns an empty PubKey.
func (sys *System) FetchZapProvider(ctx context.Context, pk nostr.PubKey) nostr.PubKey {
	if v, ok := sys.ZapProviderCache.Get(pk); ok {
		return v
	}

	pm := sys.FetchProfileMetadata(ctx, pk)

	if pm.LUD16 != "" {
		parts := strings.Split(pm.LUD16, "@")
		if len(parts) == 2 {
			url := "http://" + parts[1] + "/.well-known/lnurlp/" + parts[0]
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err == nil {
				client := &http.Client{Timeout: 10 * time.Second}
				resp, err := client.Do(req)
				if err == nil {
					defer resp.Body.Close()
					if body, err := io.ReadAll(resp.Body); err == nil {
						gj := gjson.ParseBytes(body)
						if gj.Get("allowsNostr").Type == gjson.True {
							if pk, err := nostr.PubKeyFromHex(gj.Get("nostrPubkey").Str); err == nil {
								sys.ZapProviderCache.SetWithTTL(pk, pk, time.Hour*6)
								return pk
							}
						}
					}
				}
			}
		}
	}

	sys.ZapProviderCache.SetWithTTL(pk, nostr.ZeroPK, time.Hour*2)
	return nostr.ZeroPK
}
