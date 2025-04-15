package checks

import (
	"fiatjaf.com/nostr/eventstore"
	"fiatjaf.com/nostr/eventstore/badger"
	"fiatjaf.com/nostr/eventstore/bluge"
	"fiatjaf.com/nostr/eventstore/edgedb"
	"fiatjaf.com/nostr/eventstore/lmdb"
	"fiatjaf.com/nostr/eventstore/mongo"
	"fiatjaf.com/nostr/eventstore/mysql"
	"fiatjaf.com/nostr/eventstore/postgresql"
	"fiatjaf.com/nostr/eventstore/sqlite3"
	"fiatjaf.com/nostr/eventstore/strfry"
)

// compile-time checks to ensure all backends implement Store
var (
	_ eventstore.Store = (*badger.BadgerBackend)(nil)
	_ eventstore.Store = (*lmdb.LMDBBackend)(nil)
	_ eventstore.Store = (*edgedb.EdgeDBBackend)(nil)
	_ eventstore.Store = (*postgresql.PostgresBackend)(nil)
	_ eventstore.Store = (*mongo.MongoDBBackend)(nil)
	_ eventstore.Store = (*sqlite3.SQLite3Backend)(nil)
	_ eventstore.Store = (*strfry.StrfryBackend)(nil)
	_ eventstore.Store = (*bluge.BlugeBackend)(nil)
	_ eventstore.Store = (*mysql.MySQLBackend)(nil)
)
