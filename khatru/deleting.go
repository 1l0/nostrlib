package khatru

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"fiatjaf.com/nostr"
)

func (rl *Relay) handleDeleteRequest(ctx context.Context, evt nostr.Event) error {
	// event deletion -- nip09
	for _, tag := range evt.Tags {
		if len(tag) >= 2 {
			var f nostr.Filter

			switch tag[0] {
			case "e":
				id, err := nostr.IDFromHex(tag[1])
				if err != nil {
					return fmt.Errorf("invalid 'e' tag '%s': %w", tag[1], err)
				}
				f = nostr.Filter{IDs: []nostr.ID{id}}
			case "a":
				spl := strings.Split(tag[1], ":")
				if len(spl) != 3 {
					continue
				}
				kind, err := strconv.Atoi(spl[0])
				if err != nil {
					continue
				}
				author, err := nostr.PubKeyFromHex(spl[1])
				if err != nil {
					continue
				}

				identifier := spl[2]
				f = nostr.Filter{
					Kinds:   []uint16{uint16(kind)},
					Authors: []nostr.PubKey{author},
					Tags:    nostr.TagMap{"d": []string{identifier}},
					Until:   &evt.CreatedAt,
				}
			default:
				continue
			}

			ctx := context.WithValue(ctx, internalCallKey, struct{}{})

			if nil != rl.QueryStored {
				for target := range rl.QueryStored(ctx, f) {
					// got the event, now check if the user can delete it
					acceptDeletion := target.PubKey == evt.PubKey
					var msg string
					if !acceptDeletion {
						msg = "you are not the author of this event"
					}

					if acceptDeletion {
						// delete it
						if nil != rl.DeleteEvent {
							if err := rl.DeleteEvent(ctx, target.ID); err != nil {
								return err
							}
						}

						// if it was tracked to be expired that is not needed anymore
						rl.expirationManager.removeEvent(target.ID)
					} else {
						// fail and stop here
						return fmt.Errorf("blocked: %s", msg)
					}
				}

				// don't try to query this same event again
				break
			}
		}
	}

	return nil
}
