package nostr

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"unsafe"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

func Generate() SecretKey {
	var sk SecretKey
	if _, err := io.ReadFull(rand.Reader, sk[:]); err != nil {
		panic(fmt.Errorf("failed to read random bytes when generating private key"))
	}
	return sk
}

type SecretKey [32]byte

func (sk SecretKey) String() string { return hex.EncodeToString(sk[:]) }
func (sk SecretKey) Hex() string    { return hex.EncodeToString(sk[:]) }
func (sk SecretKey) Public() PubKey { return GetPublicKey(sk) }

func SecretKeyFromHex(idh string) (SecretKey, error) {
	id := SecretKey{}

	if len(idh) != 64 {
		return id, fmt.Errorf("pubkey should be 64-char hex, got '%s'", idh)
	}
	if _, err := hex.Decode(id[:], unsafe.Slice(unsafe.StringData(idh), 64)); err != nil {
		return id, fmt.Errorf("'%s' is not valid hex: %w", idh, err)
	}

	return id, nil
}

func MustSecretKeyFromHex(idh string) SecretKey {
	id := SecretKey{}
	hex.Decode(id[:], unsafe.Slice(unsafe.StringData(idh), 64))
	return id
}

func GetPublicKey(sk [32]byte) PubKey {
	_, pk := btcec.PrivKeyFromBytes(sk[:])
	return [32]byte(pk.SerializeCompressed()[1:])
}

var ZeroPK = [32]byte{}

type PubKey [32]byte

func (pk PubKey) String() string { return hex.EncodeToString(pk[:]) }
func (pk PubKey) Hex() string    { return hex.EncodeToString(pk[:]) }

func PubKeyFromHex(pkh string) (PubKey, error) {
	pk := PubKey{}
	if len(pkh) != 64 {
		return pk, fmt.Errorf("pubkey should be 64-char hex, got '%s'", pkh)
	}
	if _, err := hex.Decode(pk[:], unsafe.Slice(unsafe.StringData(pkh), 64)); err != nil {
		return pk, fmt.Errorf("'%s' is not valid hex: %w", pkh, err)
	}
	if _, err := schnorr.ParsePubKey(pk[:]); err != nil {
		return pk, fmt.Errorf("'%s' is not a valid pubkey", pkh)
	}
	return pk, nil
}

func PubKeyFromHexCheap(pkh string) (PubKey, error) {
	pk := PubKey{}
	if len(pkh) != 64 {
		return pk, fmt.Errorf("pubkey should be 64-char hex, got '%s'", pkh)
	}
	if _, err := hex.Decode(pk[:], unsafe.Slice(unsafe.StringData(pkh), 64)); err != nil {
		return pk, fmt.Errorf("'%s' is not valid hex: %w", pkh, err)
	}

	return pk, nil
}

func MustPubKeyFromHex(pkh string) PubKey {
	pk := PubKey{}
	hex.Decode(pk[:], unsafe.Slice(unsafe.StringData(pkh), 64))
	return pk
}

func ContainsPubKey(haystack []PubKey, needle PubKey) bool {
	for _, cand := range haystack {
		if cand == needle {
			return true
		}
	}
	return false
}
