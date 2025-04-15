package bluge

import (
	"fmt"
	"strconv"

	"fiatjaf.com/nostr"
	"github.com/blugelabs/bluge"
)

func (b *BlugeBackend) SaveEvent(evt nostr.Event) error {
	id := eventIdentifier(evt.ID)
	doc := &bluge.Document{
		bluge.NewKeywordFieldBytes(id.Field(), id.Term()).Sortable().StoreValue(),
	}

	doc.AddField(bluge.NewTextField(contentField, evt.Content))
	doc.AddField(bluge.NewTextField(kindField, strconv.Itoa(int(evt.Kind))))
	doc.AddField(bluge.NewTextField(pubkeyField, evt.PubKey.Hex()[56:]))
	doc.AddField(bluge.NewNumericField(createdAtField, float64(evt.CreatedAt)))

	if err := b.writer.Update(doc.ID(), doc); err != nil {
		return fmt.Errorf("failed to write '%s' document: %w", evt.ID, err)
	}

	return nil
}
