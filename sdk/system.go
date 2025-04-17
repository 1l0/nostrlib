package sdk

import (
	"context"
	"math/rand/v2"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/eventstore/wrappers"
	"fiatjaf.com/nostr/sdk/cache"
	cache_memory "fiatjaf.com/nostr/sdk/cache/memory"
	"fiatjaf.com/nostr/sdk/dataloader"
	"fiatjaf.com/nostr/sdk/hints"
	"fiatjaf.com/nostr/sdk/hints/memoryh"
	"fiatjaf.com/nostr/sdk/kvstore"
	kvstore_memory "fiatjaf.com/nostr/sdk/kvstore/memory"
	"fiatjaf.com/nostr/eventstore"
	"fiatjaf.com/nostr/eventstore/nullstore"
)

// System represents the core functionality of the SDK, providing access to
// various caches, relays, and dataloaders for efficient Nostr operations.
//
// Usually an application should have a single global instance of this and use
// its internal Pool for all its operations.
//
// Store, KVStore and Hints are databases that should generally be persisted
// for any application that is intended to be executed more than once. By
// default they're set to in-memory stores, but ideally persisteable
// implementations should be given (some alternatives are provided in subpackages).
type System struct {
	KVStore               kvstore.KVStore
	MetadataCache         cache.Cache32[ProfileMetadata]
	RelayListCache        cache.Cache32[GenericList[Relay]]
	FollowListCache       cache.Cache32[GenericList[ProfileRef]]
	MuteListCache         cache.Cache32[GenericList[ProfileRef]]
	BookmarkListCache     cache.Cache32[GenericList[EventRef]]
	PinListCache          cache.Cache32[GenericList[EventRef]]
	BlockedRelayListCache cache.Cache32[GenericList[RelayURL]]
	SearchRelayListCache  cache.Cache32[GenericList[RelayURL]]
	TopicListCache        cache.Cache32[GenericList[Topic]]
	RelaySetsCache        cache.Cache32[GenericSets[RelayURL]]
	FollowSetsCache       cache.Cache32[GenericSets[ProfileRef]]
	TopicSetsCache        cache.Cache32[GenericSets[Topic]]
	Hints                 hints.HintsDB
	Pool                  *nostr.Pool
	RelayListRelays       *RelayStream
	FollowListRelays      *RelayStream
	MetadataRelays        *RelayStream
	FallbackRelays        *RelayStream
	JustIDRelays          *RelayStream
	UserSearchRelays      *RelayStream
	NoteSearchRelays      *RelayStream
	Store                 eventstore.Store

	Publisher wrappers.StorePublisher

	replaceableLoaders []*dataloader.Loader[nostr.PubKey, *nostr.Event]
	addressableLoaders []*dataloader.Loader[nostr.PubKey, []*nostr.Event]
}

// SystemModifier is a function that modifies a System instance.
// It's used with NewSystem to configure the system during creation.
type SystemModifier func(sys *System)

// RelayStream provides a rotating list of relay URLs.
// It's used to distribute requests across multiple relays.
type RelayStream struct {
	URLs   []string
	serial int
}

// NewRelayStream creates a new RelayStream with the provided URLs.
func NewRelayStream(urls ...string) *RelayStream {
	return &RelayStream{URLs: urls, serial: rand.Int()}
}

// Next returns the next URL in the rotation.
func (rs *RelayStream) Next() string {
	rs.serial++
	return rs.URLs[rs.serial%len(rs.URLs)]
}

// NewSystem creates a new System with default configuration,
// which can be customized using the provided modifiers.
//
// The list of provided With* modifiers isn't exhaustive and
// most internal fields of System can be modified after the System
// creation -- and in many cases one or another of these will have
// to be modified, so don't be afraid of doing that.
func NewSystem(mods ...SystemModifier) *System {
	sys := &System{
		KVStore:          kvstore_memory.NewStore(),
		RelayListRelays:  NewRelayStream("wss://purplepag.es", "wss://user.kindpag.es", "wss://relay.nos.social"),
		FollowListRelays: NewRelayStream("wss://purplepag.es", "wss://user.kindpag.es", "wss://relay.nos.social"),
		MetadataRelays:   NewRelayStream("wss://purplepag.es", "wss://user.kindpag.es", "wss://relay.nos.social"),
		FallbackRelays: NewRelayStream(
			"wss://offchain.pub",
			"wss://no.str.cr",
			"wss://relay.damus.io",
			"wss://nostr.mom",
			"wss://nos.lol",
			"wss://relay.mostr.pub",
			"wss://nostr.land",
		),
		JustIDRelays: NewRelayStream(
			"wss://cache2.primal.net/v1",
			"wss://relay.nostr.band",
		),
		UserSearchRelays: NewRelayStream(
			"wss://search.nos.today",
			"wss://nostr.wine",
			"wss://relay.nostr.band",
		),
		NoteSearchRelays: NewRelayStream(
			"wss://nostr.wine",
			"wss://relay.nostr.band",
			"wss://search.nos.today",
		),
		Hints: memoryh.NewHintDB(),
	}

	sys.Pool = nostr.NewPool(context.Background(),
		nostr.WithAuthorKindQueryMiddleware(sys.TrackQueryAttempts),
		nostr.WithEventMiddleware(sys.TrackEventHintsAndRelays),
		nostr.WithDuplicateMiddleware(sys.TrackEventRelaysD),
		nostr.WithPenaltyBox(),
	)

	for _, mod := range mods {
		mod(sys)
	}

	if sys.MetadataCache == nil {
		sys.MetadataCache = cache_memory.New[ProfileMetadata](8000)
	}
	if sys.RelayListCache == nil {
		sys.RelayListCache = cache_memory.New[GenericList[Relay]](8000)
	}

	if sys.Store == nil {
		sys.Store = &nullstore.NullStore{}
		sys.Store.Init()
	}
	sys.StoreRelay = eventstore.RelayWrapper{Store: sys.Store}

	sys.initializeReplaceableDataloaders()
	sys.initializeAddressableDataloaders()

	return sys
}

// Close releases resources held by the System.
func (sys *System) Close() {
	if sys.KVStore != nil {
		sys.KVStore.Close()
	}
	if sys.Pool != nil {
		sys.Pool.Close("sdk.System closed")
	}
}

// WithHintsDB returns a SystemModifier that sets the HintsDB.
func WithHintsDB(hdb hints.HintsDB) SystemModifier {
	return func(sys *System) {
		sys.Hints = hdb
	}
}

// WithRelayListRelays returns a SystemModifier that sets the RelayListRelays.
func WithRelayListRelays(list []string) SystemModifier {
	return func(sys *System) {
		sys.RelayListRelays.URLs = list
	}
}

// WithMetadataRelays returns a SystemModifier that sets the MetadataRelays.
func WithMetadataRelays(list []string) SystemModifier {
	return func(sys *System) {
		sys.MetadataRelays.URLs = list
	}
}

// WithFollowListRelays returns a SystemModifier that sets the FollowListRelays.
func WithFollowListRelays(list []string) SystemModifier {
	return func(sys *System) {
		sys.FollowListRelays.URLs = list
	}
}

// WithFallbackRelays returns a SystemModifier that sets the FallbackRelays.
func WithFallbackRelays(list []string) SystemModifier {
	return func(sys *System) {
		sys.FallbackRelays.URLs = list
	}
}

// WithJustIDRelays returns a SystemModifier that sets the JustIDRelays.
func WithJustIDRelays(list []string) SystemModifier {
	return func(sys *System) {
		sys.JustIDRelays.URLs = list
	}
}

// WithUserSearchRelays returns a SystemModifier that sets the UserSearchRelays.
func WithUserSearchRelays(list []string) SystemModifier {
	return func(sys *System) {
		sys.UserSearchRelays.URLs = list
	}
}

// WithNoteSearchRelays returns a SystemModifier that sets the NoteSearchRelays.
func WithNoteSearchRelays(list []string) SystemModifier {
	return func(sys *System) {
		sys.NoteSearchRelays.URLs = list
	}
}

// WithStore returns a SystemModifier that sets the Store.
func WithStore(store eventstore.Store) SystemModifier {
	return func(sys *System) {
		sys.Store = store
	}
}

// WithRelayListCache returns a SystemModifier that sets the RelayListCache.
func WithRelayListCache(cache cache.Cache32[GenericList[Relay]]) SystemModifier {
	return func(sys *System) {
		sys.RelayListCache = cache
	}
}

// WithFollowListCache returns a SystemModifier that sets the FollowListCache.
func WithFollowListCache(cache cache.Cache32[GenericList[ProfileRef]]) SystemModifier {
	return func(sys *System) {
		sys.FollowListCache = cache
	}
}

// WithMetadataCache returns a SystemModifier that sets the MetadataCache.
func WithMetadataCache(cache cache.Cache32[ProfileMetadata]) SystemModifier {
	return func(sys *System) {
		sys.MetadataCache = cache
	}
}

// WithKVStore returns a SystemModifier that sets the KVStore.
func WithKVStore(store kvstore.KVStore) SystemModifier {
	return func(sys *System) {
		if sys.KVStore != nil {
			sys.KVStore.Close()
		}
		sys.KVStore = store
	}
}
