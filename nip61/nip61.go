package nip61

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"slices"
	"strings"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip60"
	"github.com/btcsuite/btcd/btcec/v2"
)

var NutzapsNotAccepted = errors.New("user doesn't accept nutzaps")

func SendNutzap(
	ctx context.Context,
	kr nostr.Keyer,
	w *nip60.Wallet,
	pool *nostr.Pool,
	targetUserPublickey nostr.PubKey,
	getUserReadRelays func(context.Context, nostr.PubKey, int) []string,
	relays []string,
	eventId nostr.ID, // can be "" if not targeting a specific event
	amount uint64,
	message string,
) (chan nostr.PublishResult, error) {
	ie := pool.QuerySingle(ctx, relays, nostr.Filter{
		Kinds:   []nostr.Kind{10019},
		Authors: []nostr.PubKey{targetUserPublickey},
	},
		nostr.SubscriptionOptions{Label: "pre-nutzap"})
	if ie == nil {
		return nil, NutzapsNotAccepted
	}

	info := Info{}
	if err := info.ParseEvent(ie.Event); err != nil {
		return nil, err
	}

	if len(info.Mints) == 0 || info.PublicKey == nostr.ZeroPK {
		return nil, NutzapsNotAccepted
	}

	targetRelays := info.Relays
	if len(targetRelays) == 0 {
		targetRelays = getUserReadRelays(ctx, targetUserPublickey, 3)
		if len(targetRelays) == 0 {
			return nil, fmt.Errorf("no relays found for sending the nutzap")
		}
	}

	nutzap := nostr.Event{
		CreatedAt: nostr.Now(),
		Kind:      nostr.KindNutZap,
		Tags:      make(nostr.Tags, 0, 8),
	}

	nutzap.Tags = append(nutzap.Tags, nostr.Tag{"p", targetUserPublickey.Hex()})
	if eventId != nostr.ZeroID {
		nutzap.Tags = append(nutzap.Tags, nostr.Tag{"e", eventId.Hex()})
	}

	p2pk, err := btcec.ParsePubKey(append([]byte{2}, info.PublicKey[:]...))
	if err != nil {
		return nil, fmt.Errorf("invalid p2pk target '%s': %w", info.PublicKey.Hex(), err)
	}

	// check if we have enough tokens in any of these mints
	for mint := range getEligibleTokensWeHave(info.Mints, w.Tokens, amount) {
		proofs, _, err := w.Send(ctx, amount, nip60.SendOptions{
			P2PK:               p2pk,
			SpecificSourceMint: mint,
		})
		if err != nil {
			continue
		}

		// we have succeeded, now we just have to publish the event
		nutzap.Tags = append(nutzap.Tags, nostr.Tag{"u", mint})
		for _, proof := range proofs {
			proofj, _ := json.Marshal(proof)
			nutzap.Tags = append(nutzap.Tags, nostr.Tag{"proof", string(proofj)})
		}

		if err := kr.SignEvent(ctx, &nutzap); err != nil {
			return nil, fmt.Errorf("failed to sign nutzap event %s: %w", nutzap, err)
		}

		return pool.PublishMany(ctx, targetRelays, nutzap), nil
	}

	// we don't have tokens at the desired target mint, so we first have to create some
	for _, mint := range info.Mints {
		proofs, err := w.SendExternal(ctx, mint, amount, nip60.SendOptions{
			P2PK: p2pk,
		})
		if err != nil {
			if strings.Contains(err.Error(), "generate mint quote") {
				continue
			}
			return nil, fmt.Errorf("failed to send: %w", err)
		}

		// we have succeeded, now we just have to publish the event
		nutzap.Tags = append(nutzap.Tags, nostr.Tag{"u", mint})
		for _, proof := range proofs {
			proofj, _ := json.Marshal(proof)
			nutzap.Tags = append(nutzap.Tags, nostr.Tag{"proof", string(proofj)})
		}

		if err := kr.SignEvent(ctx, &nutzap); err != nil {
			return nil, fmt.Errorf("failed to sign nutzap event %s: %w", nutzap, err)
		}

		return pool.PublishMany(ctx, targetRelays, nutzap), nil
	}

	return nil, fmt.Errorf("failed to send, we don't have enough money or all mints are down")
}

func getEligibleTokensWeHave(
	theirMints []string,
	ourTokens []nip60.Token,
	targetAmount uint64,
) iter.Seq[string] {
	have := make([]uint64, len(theirMints))

	return func(yield func(string) bool) {
		for _, token := range ourTokens {
			if idx := slices.Index(theirMints, token.Mint); idx != -1 {
				have[idx] += token.Proofs.Amount()

				/*                          hardcoded estimated maximum fee,
				                            unlikely to be more than this */
				if have[idx] > targetAmount*101/100+2 {
					if !yield(token.Mint) {
						break
					}
				}
			}
		}
	}
}
