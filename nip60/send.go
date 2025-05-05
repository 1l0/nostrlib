package nip60

import (
	"context"
	"encoding/hex"
	"fmt"
	"slices"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip60/client"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/elnosh/gonuts/cashu"
	"github.com/elnosh/gonuts/cashu/nuts/nut02"
	"github.com/elnosh/gonuts/cashu/nuts/nut10"
	"github.com/elnosh/gonuts/cashu/nuts/nut11"
)

// SendOptions contains options for sending tokens
type SendOptions struct {
	SpecificSourceMint string
	P2PK               *btcec.PublicKey
	RefundTimelock     nostr.Timestamp
}

type chosenTokens struct {
	mint         string
	tokens       []Token
	tokenIndexes []int
	proofs       cashu.Proofs
	keysets      []nut02.Keyset
}

func (w *Wallet) Send(ctx context.Context, amount uint64, opts SendOptions) (cashu.Proofs, string, error) {
	if w.PublishUpdate == nil {
		return nil, "", fmt.Errorf("can't do write operations: missing PublishUpdate function")
	}

	w.tokensMu.Lock()
	defer w.tokensMu.Unlock()

	chosen, _, err := w.getProofsForSending(ctx, amount, opts.SpecificSourceMint, nil)
	if err != nil {
		return nil, "", err
	}

	swapSettings := swapSettings{}
	if opts.P2PK != nil {
		if info, err := client.GetMintInfo(ctx, chosen.mint); err != nil || !info.Nuts.Nut11.Supported {
			return nil, chosen.mint, fmt.Errorf("mint doesn't support p2pk: %w", err)
		}

		tags := nut11.P2PKTags{
			NSigs:    1,
			Locktime: 0,
			Pubkeys:  []*btcec.PublicKey{opts.P2PK},
		}
		if opts.RefundTimelock != 0 {
			tags.Refund = []*btcec.PublicKey{w.PublicKey}
			tags.Locktime = int64(opts.RefundTimelock)
		}

		swapSettings.spendingCondition = &nut10.SpendingCondition{
			Kind: nut10.P2PK,
			Data: hex.EncodeToString(opts.P2PK.SerializeCompressed()),
			Tags: nut11.SerializeP2PKTags(tags),
		}
	}

	// get new proofs
	proofsToSend, changeProofs, err := w.swapProofs(ctx, chosen.mint, chosen.proofs, amount, swapSettings)
	if err != nil {
		return nil, chosen.mint, err
	}

	he := HistoryEntry{
		event:           &nostr.Event{},
		TokenReferences: make([]TokenRef, 0, 5),
		createdAt:       nostr.Now(),
		In:              false,
		Amount:          chosen.proofs.Amount() - changeProofs.Amount(),
	}

	if err := w.saveChangeAndDeleteUsedTokens(ctx, chosen.mint, changeProofs, chosen.tokenIndexes, &he); err != nil {
		return nil, chosen.mint, err
	}

	w.Lock()
	if err := he.toEvent(ctx, w.kr, he.event); err == nil {
		w.PublishUpdate(*he.event, nil, nil, nil, true)
	}
	w.Unlock()

	return proofsToSend, chosen.mint, nil
}

func (w *Wallet) saveChangeAndDeleteUsedTokens(
	ctx context.Context,
	mintURL string,
	changeProofs cashu.Proofs,
	usedTokenIndexes []int,
	he *HistoryEntry,
) error {
	// delete spent tokens and save our change
	updatedTokens := make([]Token, 0, len(w.Tokens))

	changeToken := Token{
		mintedAt: nostr.Now(),
		Mint:     mintURL,
		Proofs:   changeProofs,
		Deleted:  make([]nostr.ID, 0, len(usedTokenIndexes)),
		event:    &nostr.Event{},
	}

	for i, token := range w.Tokens {
		if slices.Contains(usedTokenIndexes, i) {
			if token.event != nil {
				token.Deleted = append(token.Deleted, token.event.ID)

				deleteEvent := nostr.Event{
					CreatedAt: nostr.Now(),
					Kind:      5,
					Tags:      nostr.Tags{{"e", token.event.ID.Hex()}, {"k", "7375"}},
				}
				w.kr.SignEvent(ctx, &deleteEvent)

				w.Lock()
				w.PublishUpdate(deleteEvent, &token, nil, nil, false)
				w.Unlock()

				// fill in the history deleted token
				he.TokenReferences = append(he.TokenReferences, TokenRef{
					EventID:  token.event.ID,
					Created:  false,
					IsNutzap: false,
				})
			}
			continue
		}
		updatedTokens = append(updatedTokens, token)
	}

	if len(changeToken.Proofs) > 0 {
		if err := changeToken.toEvent(ctx, w.kr, changeToken.event); err != nil {
			return fmt.Errorf("failed to make change token: %w", err)
		}
		w.Lock()
		w.PublishUpdate(*changeToken.event, nil, nil, &changeToken, false)
		w.Unlock()

		// we don't have to lock tokensMu here because this function will always be called with that lock already held
		w.Tokens = append(updatedTokens, changeToken)

		// fill in the history created token
		he.TokenReferences = append(he.TokenReferences, TokenRef{
			EventID:  changeToken.event.ID,
			Created:  true,
			IsNutzap: false,
		})
	}

	return nil
}

func (w *Wallet) getProofsForSending(
	ctx context.Context,
	amount uint64,
	fromMint string,
	excludeMints []string,
) (chosenTokens, uint64, error) {
	byMint := make(map[string]chosenTokens)
	for t, token := range w.Tokens {
		if fromMint != "" && token.Mint != fromMint {
			continue
		}
		if slices.Contains(excludeMints, token.Mint) {
			continue
		}

		part, ok := byMint[token.Mint]
		if !ok {
			keysets, err := client.GetAllKeysets(ctx, token.Mint)
			if err != nil {
				return chosenTokens{}, 0, fmt.Errorf("failed to get %s keysets: %w", token.Mint, err)
			}
			part.keysets = keysets
			part.tokens = make([]Token, 0, 3)
			part.tokenIndexes = make([]int, 0, 3)
			part.proofs = make(cashu.Proofs, 0, 7)
			part.mint = token.Mint
		}

		part.tokens = append(part.tokens, token)
		part.tokenIndexes = append(part.tokenIndexes, t)
		part.proofs = append(part.proofs, token.Proofs...)
		if part.proofs.Amount() >= amount {
			// maybe we found it here
			fee := calculateFee(part.proofs, part.keysets)
			if part.proofs.Amount() >= (amount + fee) {
				// yes, we did
				return part, fee, nil
			}
		}

		byMint[token.Mint] = part
	}

	// if we got here it's because we didn't get enough proofs from the same mint
	return chosenTokens{}, 0, fmt.Errorf("not enough proofs found from the same mint")
}
