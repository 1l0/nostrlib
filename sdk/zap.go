package sdk

import (
	"context"
	"crypto/sha256"
	"io"
	"net/http"
	"strings"
	"time"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip60"
	"fiatjaf.com/nostr/nip60/client"
	"github.com/btcsuite/btcd/btcec/v2"
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

// FetchMintKeys fetches the active keyset from the given mint URL and parses the keys.
// It uses a cache to avoid repeated fetches.
func (sys *System) FetchMintKeys(ctx context.Context, mintURL string) (map[uint64]*btcec.PublicKey, error) {
	hash := sha256.Sum256([]byte(mintURL))
	if v, ok := sys.MintKeysCache.Get(hash); ok {
		return v, nil
	}

	keyset, err := client.GetActiveKeyset(ctx, mintURL)
	if err != nil {
		return nil, err
	}

	ksKeys, err := nip60.ParseKeysetKeys(keyset.Keys)
	if err != nil {
		return nil, err
	}

	sys.MintKeysCache.SetWithTTL(hash, ksKeys, time.Hour*6)
	return ksKeys, nil
}
