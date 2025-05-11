package main

import (
	"fmt"
	"net/http"

	"fiatjaf.com/nostr/eventstore/badger"
	"fiatjaf.com/nostr/khatru"
)

func main() {
	relay := khatru.NewRelay()

	db := &badger.BadgerBackend{Path: "/tmp/khatru-badgern-tmp"}
	if err := db.Init(); err != nil {
		panic(err)
	}

	relay.UseEventstore(db, 400)

	relay.Negentropy = true

	fmt.Println("running on :3334")
	http.ListenAndServe(":3334", relay)
}
