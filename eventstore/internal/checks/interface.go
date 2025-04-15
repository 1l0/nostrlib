package checks

import (
	"fiatjaf.com/nostr/eventstore"
	"fiatjaf.com/nostr/eventstore/badger"
	"fiatjaf.com/nostr/eventstore/bluge"
	"fiatjaf.com/nostr/eventstore/lmdb"
	"fiatjaf.com/nostr/eventstore/strfry"
)

// compile-time checks to ensure all backends implement Store
var (
	_ eventstore.Store = (*badger.BadgerBackend)(nil)
	_ eventstore.Store = (*lmdb.LMDBBackend)(nil)
	_ eventstore.Store = (*strfry.StrfryBackend)(nil)
	_ eventstore.Store = (*bluge.BlugeBackend)(nil)
)
