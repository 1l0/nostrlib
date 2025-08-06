package test

import (
	"slices"
	"testing"

	"fiatjaf.com/nostr"
	"fiatjaf.com/nostr/eventstore"
	"github.com/stretchr/testify/require"
)

func basicTest(t *testing.T, db eventstore.Store) {
	err := db.Init()
	require.NoError(t, err)

	// from basic-test.patch
	{
		// create test events with different tags and authors
		pk3 := nostr.GetPublicKey(sk3)
		pk4 := nostr.GetPublicKey(sk4)

		events := []nostr.Event{
			// event with 'e' tag
			{
				CreatedAt: 100,
				Content:   "event with e tag",
				Tags:      nostr.Tags{{"e", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
				Kind:      1,
			},
			// event with 'q' tag
			{
				CreatedAt: 101,
				Content:   "event with q tag",
				Tags:      nostr.Tags{{"q", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
				Kind:      1,
			},
			// event with 'p' tag and kind 3
			{
				CreatedAt: 102,
				Content:   "event with p tag kind 3",
				Tags:      nostr.Tags{{"p", pk3.Hex()}},
				Kind:      3,
			},
			// event with 'p' tag and kind 7
			{
				CreatedAt: 103,
				Content:   "event with p tag kind 7",
				Tags:      nostr.Tags{{"p", pk4.Hex()}},
				Kind:      7,
			},
			// event from author pk3 with kind 1
			{
				CreatedAt: 104,
				Content:   "event from pk3 kind 1",
				Tags:      nostr.Tags{},
				Kind:      1,
			},
			// event from author pk4 with kind 3
			{
				CreatedAt: 105,
				Content:   "event from pk4 kind 3",
				Tags:      nostr.Tags{},
				Kind:      3,
			},
		}

		// sign events with appropriate keys
		events[0].Sign(sk3)
		events[1].Sign(sk3)
		events[2].Sign(sk3)
		events[3].Sign(sk3)
		events[4].Sign(sk3)
		events[5].Sign(sk4)

		// save all events
		for _, evt := range events {
			err = db.SaveEvent(evt)
			require.NoError(t, err)
		}

		// test 0: query all
		{
			results := slices.Collect(db.QueryEvents(nostr.Filter{}, 1000))
			require.NoError(t, err)
			require.Len(t, results, 6)
		}

		// test 1: query by 'e' tag
		{
			results := slices.Collect(db.QueryEvents(nostr.Filter{
				Tags: nostr.TagMap{"e": []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
			}, 1000))
			require.NoError(t, err)
			require.Len(t, results, 1)
			require.Equal(t, events[0].ID, results[0].ID, "e tag query error")
		}

		// test 2: query by 'q' tag
		{
			results := slices.Collect(db.QueryEvents(nostr.Filter{
				Tags: nostr.TagMap{"q": []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
			}, 1000))
			require.NoError(t, err)
			require.Len(t, results, 1)
			require.Equal(t, events[1].ID, results[0].ID, "q tag query error")
		}

		// test 3: query by 'p' tag + kind
		{
			results := slices.Collect(db.QueryEvents(nostr.Filter{
				Tags:  nostr.TagMap{"p": []string{pk3.Hex()}},
				Kinds: []nostr.Kind{3},
			}, 1000))
			require.NoError(t, err)
			require.Len(t, results, 1)
			require.Equal(t, events[2].ID, results[0].ID, "p tag + kind query error")
		}

		// test 4: query by author + kind
		{
			results := slices.Collect(db.QueryEvents(nostr.Filter{
				Authors: []nostr.PubKey{pk4},
				Kinds:   []nostr.Kind{3},
			}, 1000))
			require.NoError(t, err)
			require.Len(t, results, 1)
			require.Equal(t, events[5].ID, results[0].ID, "author + kind query error")
		}
	}

	// from another-basic-test.patch
	{
		evt1 := nostr.Event{
			CreatedAt: nostr.Now(),
			Content:   "two tags",
			Tags: nostr.Tags{
				{"e", "f355341a03672c5b136a6002fb4b69ad52111a1646638771c3995fc0a4db2a78"},
				{"q", "f355341a03672c5b136a6002fb4b69ad52111a1646638771c3995fc0a4db2a78"},
			},
			Kind: 23122,
		}
		evt1.Sign(sk3)
		require.NoError(t, db.SaveEvent(evt1))

		{
			results := slices.Collect(db.QueryEvents(nostr.Filter{
				Tags: nostr.TagMap{"e": []string{"f355341a03672c5b136a6002fb4b69ad52111a1646638771c3995fc0a4db2a78"}},
			}, 1000))
			require.NoError(t, err)
			require.Len(t, results, 1)
			require.ElementsMatch(t,
				[]nostr.Event{evt1},
				results,
				"querying by 'e' tag")
		}
		{
			results := slices.Collect(db.QueryEvents(nostr.Filter{
				Tags: nostr.TagMap{"q": []string{"f355341a03672c5b136a6002fb4b69ad52111a1646638771c3995fc0a4db2a78"}},
			}, 1000))
			require.NoError(t, err)
			require.Len(t, results, 1)
			require.ElementsMatch(t,
				[]nostr.Event{evt1},
				results,
				"querying by 'q' tag")
		}

		evt2 := nostr.Event{
			CreatedAt: nostr.Now(),
			Content:   "e tag",
			Tags: nostr.Tags{
				{"e", "f355341a03672c5b136a6002fb4b69ad52111a1646638771c3995fc0a4db2a78"},
			},
			Kind: 23122,
		}
		evt2.Sign(sk3)
		require.NoError(t, db.SaveEvent(evt2))

		evt3 := nostr.Event{
			CreatedAt: nostr.Now(),
			Content:   "q tag",
			Tags: nostr.Tags{
				{"q", "f355341a03672c5b136a6002fb4b69ad52111a1646638771c3995fc0a4db2a78"},
			},
			Kind: 23122,
		}
		evt3.Sign(sk3)
		require.NoError(t, db.SaveEvent(evt3))

		{
			results := slices.Collect(db.QueryEvents(nostr.Filter{
				Tags: nostr.TagMap{"e": []string{"f355341a03672c5b136a6002fb4b69ad52111a1646638771c3995fc0a4db2a78"}},
			}, 1000))
			require.NoError(t, err)
			require.Len(t, results, 2)
			require.ElementsMatch(t,
				[]nostr.Event{evt1, evt2},
				results,
				"querying by 'e' tag")
		}
		{
			results := slices.Collect(db.QueryEvents(nostr.Filter{
				Tags: nostr.TagMap{"q": []string{"f355341a03672c5b136a6002fb4b69ad52111a1646638771c3995fc0a4db2a78"}},
			}, 1000))
			require.NoError(t, err)
			require.Len(t, results, 2)
			require.ElementsMatch(t,
				[]nostr.Event{evt1, evt3},
				results,
				"querying by 'q' tag")
		}
	}

	// test ReplaceEvent()
	{
		pk3 := nostr.GetPublicKey(sk3)
		originalProfile := nostr.Event{
			CreatedAt: 200,
			Content:   `{"name":"original","about":"original profile"}`,
			Tags:      nostr.Tags{},
			Kind:      0,
		}
		originalProfile.Sign(sk3)

		err = db.ReplaceEvent(originalProfile)
		require.NoError(t, err)

		// verify
		results := slices.Collect(db.QueryEvents(nostr.Filter{
			Authors: []nostr.PubKey{pk3},
			Kinds:   []nostr.Kind{0},
		}, 1000))
		require.Len(t, results, 1)
		require.Equal(t, originalProfile.ID, results[0].ID)

		// create newer profile event
		newProfile := nostr.Event{
			CreatedAt: 300, // newer timestamp
			Content:   `{"name":"updated","about":"updated profile"}`,
			Tags:      nostr.Tags{},
			Kind:      0,
		}
		newProfile.Sign(sk3)

		// replace with newer event
		err = db.ReplaceEvent(newProfile)
		require.NoError(t, err)

		// verify only the newer event exists
		results = slices.Collect(db.QueryEvents(nostr.Filter{
			Authors: []nostr.PubKey{pk3},
			Kinds:   []nostr.Kind{0},
		}, 1000))
		require.Len(t, results, 1)
		require.Equal(t, newProfile.ID, results[0].ID)

		// try to replace with older event (should be ignored)
		olderProfile := nostr.Event{
			CreatedAt: 250, // older than current
			Content:   `{"name":"older","about":"older profile"}`,
			Tags:      nostr.Tags{},
			Kind:      0,
		}
		olderProfile.Sign(sk3)

		err = db.ReplaceEvent(olderProfile)
		require.NoError(t, err)

		// verify the newer event is still there
		results = slices.Collect(db.QueryEvents(nostr.Filter{
			Authors: []nostr.PubKey{pk3},
			Kinds:   []nostr.Kind{0},
		}, 1000))
		require.Len(t, results, 1)
		require.Equal(t, newProfile.ID, results[0].ID)

		// test addressable event (kind 30023 - article)
		articleV1 := nostr.Event{
			CreatedAt: 400,
			Content:   "first version of article",
			Tags:      nostr.Tags{{"d", "my-article"}}, // addressable identifier
			Kind:      30023,                           // article - addressable
		}
		articleV1.Sign(sk3)

		err = db.ReplaceEvent(articleV1)
		require.NoError(t, err)

		// verify article was saved
		results = slices.Collect(db.QueryEvents(nostr.Filter{
			Authors: []nostr.PubKey{pk3},
			Kinds:   []nostr.Kind{30023},
			Tags:    nostr.TagMap{"d": []string{"my-article"}},
		}, 1000))
		require.Len(t, results, 1)
		require.Equal(t, articleV1.ID, results[0].ID)

		// create updated version of same article
		articleV2 := nostr.Event{
			CreatedAt: 500,
			Content:   "second version of article",
			Tags:      nostr.Tags{{"d", "my-article"}}, // same identifier
			Kind:      30023,
		}
		articleV2.Sign(sk3)

		err = db.ReplaceEvent(articleV2)
		require.NoError(t, err)

		// verify only the newer version exists
		results = slices.Collect(db.QueryEvents(nostr.Filter{
			Authors: []nostr.PubKey{pk3},
			Kinds:   []nostr.Kind{30023},
			Tags:    nostr.TagMap{"d": []string{"my-article"}},
		}, 1000))
		require.Len(t, results, 1)
		require.Equal(t, articleV2.ID, results[0].ID)
		require.Equal(t, "second version of article", results[0].Content)

		// create different article with different d tag
		differentArticle := nostr.Event{
			CreatedAt: 600,
			Content:   "different article",
			Tags:      nostr.Tags{{"d", "other-article"}}, // different identifier
			Kind:      30023,
		}
		differentArticle.Sign(sk3)

		err = db.ReplaceEvent(differentArticle)
		require.NoError(t, err)

		// verify both articles exist (different d tags)
		results = slices.Collect(db.QueryEvents(nostr.Filter{
			Authors: []nostr.PubKey{pk3},
			Kinds:   []nostr.Kind{30023},
		}, 1000))
		require.Len(t, results, 2)
	}
}
