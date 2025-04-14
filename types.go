package nostr

import (
	"encoding/hex"
	"fmt"
	"unsafe"
)

type PubKey [32]byte

func (pk PubKey) String() string { return hex.EncodeToString(pk[:]) }

func PubKeyFromHex(pkh string) (PubKey, error) {
	pk := PubKey{}
	if len(pkh) != 64 {
		return pk, fmt.Errorf("pubkey should be 64-char hex, got '%s'", pkh)
	}
	if _, err := hex.Decode(pk[:], unsafe.Slice(unsafe.StringData(pkh), 64)); err != nil {
		return pk, fmt.Errorf("'%s' is not valid hex: %w", pkh, err)
	}

	if !IsValidPublicKey(pk) {
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

type ID [32]byte

func (id ID) String() string { return hex.EncodeToString(id[:]) }

func IDFromHex(idh string) (ID, error) {
	id := ID{}

	if len(idh) != 64 {
		return id, fmt.Errorf("pubkey should be 64-char hex, got '%s'", idh)
	}
	if _, err := hex.Decode(id[:], unsafe.Slice(unsafe.StringData(idh), 64)); err != nil {
		return id, fmt.Errorf("'%s' is not valid hex: %w", idh, err)
	}

	return id, nil
}

func MustIDFromHex(idh string) ID {
	id := ID{}
	hex.Decode(id[:], unsafe.Slice(unsafe.StringData(idh), 64))
	return id
}
