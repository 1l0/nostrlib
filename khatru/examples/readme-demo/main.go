package main

import (
	"context"
	"fmt"
	"iter"
	"log"
	"net/http"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/khatru"
	"fiatjaf.com/nostr/khatru/policies"
)

func main() {
	// create the relay instance
	relay := khatru.NewRelay()

	// set up some basic properties (will be returned on the NIP-11 endpoint)
	relay.Info.Name = "my relay"
	relay.Info.PubKey = "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
	relay.Info.Description = "this is my custom relay"
	relay.Info.Icon = "https://external-content.duckduckgo.com/iu/?u=https%3A%2F%2Fliquipedia.net%2Fcommons%2Fimages%2F3%2F35%2FSCProbe.jpg&f=1&nofb=1&ipt=0cbbfef25bce41da63d910e86c3c343e6c3b9d63194ca9755351bb7c2efa3359&ipo=images"

	// you must bring your own storage scheme -- if you want to have any
	store := make(map[nostr.ID]nostr.Event, 120)

	// set up the basic relay functions
	relay.StoreEvent = func(ctx context.Context, event nostr.Event) error {
		store[event.ID] = event
		return nil
	}
	relay.QueryStored = func(ctx context.Context, filter nostr.Filter) iter.Seq[nostr.Event] {
		return func(yield func(nostr.Event) bool) {
			for _, evt := range store {
				if filter.Matches(evt) {
					yield(evt)
				}
			}
		}
	}
	relay.DeleteEvent = func(ctx context.Context, id nostr.ID) error {
		delete(store, id)
		return nil
	}

	// there are many other configurable things you can set
	relay.OnEvent = policies.SeqEvent(
		// built-in policies
		policies.ValidateKind,
		policies.PreventLargeTags(100),

		// define your own policies
		func(ctx context.Context, event nostr.Event) (reject bool, msg string) {
			if event.PubKey == nostr.MustPubKeyFromHex("fa984bd7dbb282f07e16e7ae87b26a2a7b9b90b7246a44771f0cf5ae58018f52") {
				return true, "we don't allow this person to write here"
			}
			return false, "" // anyone else can
		},
	)

	// you can request auth by rejecting an event or a request with the prefix "auth-required: "
	relay.OnRequest = policies.SeqRequest(
		// built-in policies
		policies.NoComplexFilters,

		// define your own policies
		func(ctx context.Context, filter nostr.Filter) (reject bool, msg string) {
			if authed, is := khatru.GetAuthed(ctx); !is {
				log.Printf("request from %s\n", authed)
				return false, ""
			}
			return true, "auth-required: only authenticated users can read from this relay"
			// (this will cause an AUTH message to be sent and then a CLOSED message such that clients can
			//  authenticate and then request again)
		},
	)
	// check the docs for more goodies!

	mux := relay.Router()
	// set up other http handlers
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/html")
		fmt.Fprintf(w, `<b>welcome</b> to my relay!`)
	})

	// start the server
	fmt.Println("running on :3334")
	http.ListenAndServe(":3334", relay)
}
