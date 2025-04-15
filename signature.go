//go:build !libsecp256k1

package nostr

import (
	"crypto/sha256"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// Verify checks if the event signature is valid for the given event.
// It won't look at the ID field, instead it will recompute the id from the entire event body.
// Returns true if the signature is valid, false otherwise.
func (evt Event) VerifySignature() bool {
	// read and check pubkey
	pubkey, err := schnorr.ParsePubKey(evt.PubKey[:])
	if err != nil {
		return false
	}

	// read signature
	sig, err := schnorr.ParseSignature(evt.Sig[:])
	if err != nil {
		return false
	}

	// check signature
	hash := sha256.Sum256(evt.Serialize())
	return sig.Verify(hash[:], pubkey)
}

// Sign signs an event with a given privateKey.
//
// It sets the event's ID, PubKey, and Sig fields.
//
// Returns an error if the private key is invalid or if signing fails.
func (evt *Event) Sign(secretKey [32]byte) error {
	if evt.Tags == nil {
		evt.Tags = make(Tags, 0)
	}

	sk, pk := btcec.PrivKeyFromBytes(secretKey[:])
	pkBytes := pk.SerializeCompressed()[1:]
	evt.PubKey = [32]byte(pkBytes)

	h := sha256.Sum256(evt.Serialize())
	sig, err := schnorr.Sign(sk, h[:], schnorr.FastSign())
	if err != nil {
		return err
	}

	evt.ID = h
	sigb := sig.Serialize()
	evt.Sig = [64]byte(sigb)

	return nil
}
