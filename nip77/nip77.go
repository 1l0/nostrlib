package nip77

import (
	"context"
	"fmt"
	"sync"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip77/negentropy"
	"fiatjaf.com/nostr/nip77/negentropy/storage/vector"
)

type direction struct {
	label string // for debugging only
	from  nostr.Querier
	to    nostr.Publisher
	items chan nostr.ID
}

func NegentropySync(
	ctx context.Context,
	filter nostr.Filter,
	relayUrl string,

	// where our local events will be read from.
	// if it is nil the sync will be unidirectional: download-only.
	source nostr.Querier,

	// where new events received from the relay will be written to.
	// if it is nil the sync will be unidirectional: upload-only.
	// it can also be a nostr.QuerierPublisher in case source isn't provided
	// and you need a download-only sync that respects local data.
	target nostr.Publisher,
) error {
	id := "nl-tmp" // for now we can't have more than one subscription in the same connection

	vec := vector.New()
	neg := negentropy.New(vec, 1024*1024, source != nil, target != nil)

	// connect to relay
	var err error
	errch := make(chan error)
	var relay *nostr.Relay
	relay, err = nostr.RelayConnect(ctx, relayUrl, nostr.RelayOptions{
		CustomHandler: func(data string) {
			envelope := ParseNegMessage(data)
			if envelope == nil {
				return
			}
			switch env := envelope.(type) {
			case *OpenEnvelope, *CloseEnvelope:
				errch <- fmt.Errorf("unexpected %s received from relay", env.Label())
				return
			case *ErrorEnvelope:
				errch <- fmt.Errorf("relay returned a %s: %s", env.Label(), env.Reason)
				return
			case *MessageEnvelope:
				nextmsg, err := neg.Reconcile(env.Message)
				if err != nil {
					errch <- fmt.Errorf("failed to reconcile: %w", err)
					return
				}

				if nextmsg != "" {
					msgb, _ := MessageEnvelope{id, nextmsg}.MarshalJSON()
					relay.Write(msgb)
				}
			}
		},
	})
	if err != nil {
		return err
	}

	// setup sync flows: up, down or both
	directions := make([]direction, 0, 2)
	if source != nil {
		directions = append(directions, direction{
			from:  source,
			to:    relay,
			items: neg.Haves,
		})
	}
	if target != nil {
		directions = append(directions, direction{
			from:  relay,
			to:    target,
			items: neg.HaveNots,
		})
	}

	// fill our local vector
	var usedSource nostr.Querier
	if source != nil {
		for evt := range source.QueryEvents(filter) {
			vec.Insert(evt.CreatedAt, evt.ID)
		}
		usedSource = source
	}
	if target != nil {
		if targetSource, ok := target.(nostr.Querier); ok && targetSource != usedSource {
			for evt := range source.QueryEvents(filter) {
				vec.Insert(evt.CreatedAt, evt.ID)
			}
		}
	}
	vec.Seal()

	// kickstart the process
	msg := neg.Start()
	open, _ := OpenEnvelope{id, filter, msg}.MarshalJSON()
	err = relay.WriteWithError(open)
	if err != nil {
		return fmt.Errorf("failed to write to relay: %w", err)
	}

	defer func() {
		clse, _ := CloseEnvelope{id}.MarshalJSON()
		relay.Write(clse)
	}()

	wg := sync.WaitGroup{}
	pool := sync.Pool{
		New: func() any { return make([]nostr.ID, 0, 50) },
	}

	for _, dir := range directions {
		wg.Add(1)
		go func(dir direction) {
			fmt.Println("> dir", dir.label)

			defer wg.Done()

			seen := make(map[nostr.ID]struct{})

			doSync := func(ids []nostr.ID) {
				defer wg.Done()
				defer pool.Put(ids)

				if len(ids) == 0 {
					return
				}
				for evt := range dir.from.QueryEvents(nostr.Filter{IDs: ids}) {
					dir.to.Publish(ctx, evt)
				}
			}

			ids := pool.Get().([]nostr.ID)
			for item := range dir.items {
				fmt.Println(">>>", item)
				if _, ok := seen[item]; ok {
					continue
				}
				seen[item] = struct{}{}

				fmt.Println(">>>>>", 0)
				ids = append(ids, item)
				if len(ids) == 50 {
					wg.Add(1)
					go doSync(ids)
					fmt.Println(">>>>>", 1)
					ids = pool.Get().([]nostr.ID)
				}
				fmt.Println(">>>>>", 2)
			}
			fmt.Println("> ?")
			wg.Add(1)
			doSync(ids)
		}(dir)
	}

	go func() {
		wg.Wait()
		errch <- nil
	}()

	err = <-errch
	if err != nil {
		return err
	}

	return nil
}
