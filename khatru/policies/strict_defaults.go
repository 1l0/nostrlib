package policies

import (
	"time"

	"fiatjaf.com/nostr"
)

var EventRejectionStrictDefaults = SeqEvent(
	RejectEventsWithBase64Media,
	PreventLargeTags(100),
	PreventTooManyIndexableTags(12, []nostr.Kind{3}, nil),
	PreventTooManyIndexableTags(1200, nil, []nostr.Kind{3}),
	PreventLargeContent(5000),
	EventIPRateLimiter(2, time.Minute*3, 10),
)

var RequestRejectionStrictDefaults = SeqRequest(
	NoComplexFilters,
	NoSearchQueries,
	FilterIPRateLimiter(20, time.Minute, 100),
)

var ConnectionRejectionStrictDefaults = ConnectionRateLimiter(1, time.Minute*5, 100)
