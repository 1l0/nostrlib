package bluge

import (
	"fiatjaf.com/nostr"
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
	return id[:]
}
