package sdk

import (
	"context"

	"fiatjaf.com/nostrlib"
)

func (sys *System) SearchUsers(ctx context.Context, query string) []ProfileMetadata {
	limit := 10
	profiles := make([]ProfileMetadata, 0, limit*len(sys.UserSearchRelays.URLs))

	for ie := range sys.Pool.FetchMany(ctx, sys.UserSearchRelays.URLs, nostr.Filter{
		Search: query,
		Limit:  limit,
	}, nostr.WithLabel("search")) {
		m, _ := ParseMetadata(ie.Event)
		profiles = append(profiles, m)
	}

	return profiles
}
