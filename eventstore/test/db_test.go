package test

import (
	"context"
	"os"
	"testing"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/eventstore"
	"fiatjaf.com/nostr/eventstore/lmdb"
	"fiatjaf.com/nostr/eventstore/slicestore"
)

var (
	dbpath = "/tmp/eventstore-test"
	sk3    = nostr.MustSecretKeyFromHex("0000000000000000000000000000000000000000000000000000000000000003")
	sk4    = nostr.MustSecretKeyFromHex("0000000000000000000000000000000000000000000000000000000000000004")
)

var ctx = context.Background()

var tests = []struct {
	name string
	run  func(*testing.T, eventstore.Store)
}{
	{"basic", basicTest},
	{"first", runFirstTestOn},
	{"second", runSecondTestOn},
	{"manyauthors", manyAuthorsTest},
	{"unbalanced", unbalancedTest},
}

func TestSliceStore(t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) { test.run(t, &slicestore.SliceStore{}) })
	}
}

func TestLMDB(t *testing.T) {
	for _, test := range tests {
		os.RemoveAll(dbpath + "lmdb")
		t.Run(test.name, func(t *testing.T) { test.run(t, &lmdb.LMDBBackend{Path: dbpath + "lmdb"}) })
	}
}
