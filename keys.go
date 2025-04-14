package nostr

import (
	"crypto/rand"
	"fmt"
	"io"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

func GeneratePrivateKey() [32]byte {
	var sk [32]byte
	if _, err := io.ReadFull(rand.Reader, sk[:]); err != nil {
		panic(fmt.Errorf("failed to read random bytes when generating private key"))
	}
	return sk
}

func GetPublicKey(sk [32]byte) PubKey {
	_, pk := btcec.PrivKeyFromBytes(sk[:])
	return [32]byte(pk.SerializeCompressed()[1:])
}

func IsValidPublicKey(pk [32]byte) bool {
	_, err := schnorr.ParsePubKey(pk[:])
	return err == nil
}
