package bluge

import (
	"fiatjaf.com/nostr"
	"github.com/templexxx/xhex"
)

const (
	contentField   = "c"
	kindField      = "k"
	createdAtField = "a"
	pubkeyField    = "p"
)

type eventIdentifier nostr.ID

const idField = "i"

func (id eventIdentifier) Field() string {
	return idField
}

func (id eventIdentifier) Term() []byte {
	idhex := make([]byte, 64)
	xhex.Encode(idhex, id[:])
	return idhex
}
