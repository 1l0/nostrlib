package nip77

import (
	"context"
	"fmt"
	"sync"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/nip77/negentropy"
	"fiatjaf.com/nostr/nip77/negentropy/storage/vector"
)

type Direction struct {
	From  nostr.Querier
	To    nostr.Publisher
	Items chan nostr.ID
}

func NegentropySync(
	ctx context.Context,

	relayUrl string,
	filter nostr.Filter,

	// where our local events will be read from.
	// if it is nil the sync will be unidirectional: download-only.
	source nostr.Querier,

	// where new events received from the relay will be written to.
	// if it is nil the sync will be unidirectional: upload-only.
	// it can also be a nostr.QuerierPublisher in case source isn't provided
	// and you need a download-only sync that respects local data.
	target nostr.Publisher,

	// handle ids received on each direction, usually called with Sync() so the corresponding events are
	// fetched from the source and published to the target
	handle func(ctx context.Context, wg *sync.WaitGroup, directions []Direction),
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
	directions := make([]Direction, 0, 2)
	if source != nil {
		directions = append(directions, Direction{
			From:  source,
			To:    relay,
			Items: neg.Haves,
		})
	}
	if target != nil {
		directions = append(directions, Direction{
			From:  relay,
			To:    target,
			Items: neg.HaveNots,
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

	// handle emitted events
	wg.Go(func() { handle(ctx, &wg, directions) })

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

func SyncEventsFromIDs(ctx context.Context, wg *sync.WaitGroup, directions []Direction) {
	pool := sync.Pool{
		New: func() any { return make([]nostr.ID, 0, 50) },
	}

	for _, dir := range directions {
		wg.Go(func() {
			seen := make(map[nostr.ID]struct{})

			doSync := func(ids []nostr.ID) {
				defer pool.Put(ids)

				if len(ids) == 0 {
					return
				}
				for evt := range dir.From.QueryEvents(nostr.Filter{IDs: ids}) {
					dir.To.Publish(ctx, evt)
				}
			}

			ids := pool.Get().([]nostr.ID)
			for item := range dir.Items {
				if _, ok := seen[item]; ok {
					continue
				}
				seen[item] = struct{}{}

				ids = append(ids, item)
				if len(ids) == 50 {
					wg.Add(1)
					wg.Go(func() { doSync(ids) })
					ids = pool.Get().([]nostr.ID)
				}
			}
			wg.Go(func() { doSync(ids) })
		})
	}
}
