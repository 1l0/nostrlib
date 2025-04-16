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
	label  string
	items  chan nostr.ID
	source nostr.QuerierPublisher
	target nostr.QuerierPublisher
}

type Direction int

const (
	Up   = 0
	Down = 1
	Both = 2
)

func NegentropySync(
	ctx context.Context,
	store nostr.QuerierPublisher,
	url string,
	filter nostr.Filter,
	dir Direction,
) error {
	id := "nl-tmp" // for now we can't have more than one subscription in the same connection

	vec := vector.New()
	neg := negentropy.New(vec, 1024*1024)
	ch, err := store.QueryEvents(ctx, filter)
	if err != nil {
		return err
	}

	for evt := range ch {
		vec.Insert(evt.CreatedAt, evt.ID)
	}
	vec.Seal()

	result := make(chan error)

	var r *nostr.Relay
	r, err = nostr.RelayConnect(ctx, url, nostr.RelayOptions{
		CustomHandler: func(data string) {
			envelope := ParseNegMessage(data)
			if envelope == nil {
				return
			}
			switch env := envelope.(type) {
			case *OpenEnvelope, *CloseEnvelope:
				result <- fmt.Errorf("unexpected %s received from relay", env.Label())
				return
			case *ErrorEnvelope:
				result <- fmt.Errorf("relay returned a %s: %s", env.Label(), env.Reason)
				return
			case *MessageEnvelope:
				nextmsg, err := neg.Reconcile(env.Message)
				if err != nil {
					result <- fmt.Errorf("failed to reconcile: %w", err)
					return
				}

				if nextmsg != "" {
					msgb, _ := MessageEnvelope{id, nextmsg}.MarshalJSON()
					r.Write(msgb)
				}
			}
		},
	})
	if err != nil {
		return err
	}

	msg := neg.Start()
	open, _ := OpenEnvelope{id, filter, msg}.MarshalJSON()
	err = <-r.Write(open)
	if err != nil {
		return fmt.Errorf("failed to write to relay: %w", err)
	}

	defer func() {
		clse, _ := CloseEnvelope{id}.MarshalJSON()
		r.Write(clse)
	}()

	wg := sync.WaitGroup{}
	pool := sync.Pool{
		New: func() any { return make([]nostr.ID, 0, 50) },
	}

	// Define sync directions
	directions := [][]direction{
		{{"up", neg.Haves, store, r}},
		{{"down", neg.HaveNots, r, store}},
		{{"up", neg.Haves, store, r}, {"down", neg.HaveNots, r, store}},
	}

	for _, dir := range directions[dir] {
		wg.Add(1)
		go func(dir direction) {
			defer wg.Done()

			seen := make(map[nostr.ID]struct{})

			doSync := func(ids []nostr.ID) {
				defer wg.Done()
				defer pool.Put(ids)

				if len(ids) == 0 {
					return
				}
				evtch, err := dir.source.QueryEvents(ctx, nostr.Filter{IDs: ids})
				if err != nil {
					result <- fmt.Errorf("error querying source on %s: %w", dir.label, err)
					return
				}
				for evt := range evtch {
					dir.target.Publish(ctx, evt)
				}
			}

			ids := pool.Get().([]nostr.ID)
			for item := range dir.items {
				if _, ok := seen[item]; ok {
					continue
				}
				seen[item] = struct{}{}

				ids = append(ids, item)
				if len(ids) == 50 {
					wg.Add(1)
					go doSync(ids)
					ids = pool.Get().([]nostr.ID)
				}
			}
			wg.Add(1)
			doSync(ids)
		}(dir)
	}

	go func() {
		wg.Wait()
		result <- nil
	}()

	err = <-result
	if err != nil {
		return err
	}

	return nil
}
