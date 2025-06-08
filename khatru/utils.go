package khatru

import (
	"context"

	"fiatjaf.com/nostr"
)

const (
	wsKey = iota
	subscriptionIdKey
	nip86HeaderAuthKey
	internalCallKey
)

func RequestAuth(ctx context.Context) {
	ws := GetConnection(ctx)
	ws.authLock.Lock()
	if ws.Authed == nil {
		ws.Authed = make(chan struct{})
	}
	ws.authLock.Unlock()
	ws.WriteJSON(nostr.AuthEnvelope{Challenge: &ws.Challenge})
}

func GetConnection(ctx context.Context) *WebSocket {
	wsi := ctx.Value(wsKey)
	if wsi != nil {
		return wsi.(*WebSocket)
	}
	return nil
}

func GetAuthed(ctx context.Context) (nostr.PubKey, bool) {
	if conn := GetConnection(ctx); conn != nil {
		return conn.AuthedPublicKey, conn.AuthedPublicKey != nostr.ZeroPK
	}
	if nip86Auth := ctx.Value(nip86HeaderAuthKey); nip86Auth != nil {
		return nip86Auth.(nostr.PubKey), true
	}
	return nostr.ZeroPK, false
}

// IsInternalCall returns true when a call to QueryEvents, for example, is being made because of a deletion
// or expiration request.
func IsInternalCall(ctx context.Context) bool {
	return ctx.Value(internalCallKey) != nil
}

func GetIP(ctx context.Context) string {
	conn := GetConnection(ctx)
	if conn == nil {
		return ""
	}

	return GetIPFromRequest(conn.Request)
}

func GetSubscriptionID(ctx context.Context) string {
	return ctx.Value(subscriptionIdKey).(string)
}

func SendNotice(ctx context.Context, msg string) {
	GetConnection(ctx).WriteJSON(nostr.NoticeEnvelope(msg))
}
