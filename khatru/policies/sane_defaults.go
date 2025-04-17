package policies

import (
	"time"

	"fiatjaf.com/nostr/khatru"
)

func ApplySaneDefaults(relay *khatru.Relay) {
	relay.OnEvent = SeqEvent(
		RejectEventsWithBase64Media,
		EventIPRateLimiter(2, time.Minute*3, 10),
	)

	relay.OnRequest = SeqRequest(
		NoComplexFilters,
		FilterIPRateLimiter(20, time.Minute, 100),
	)

	relay.RejectConnection = ConnectionRateLimiter(1, time.Minute*5, 100)
}
