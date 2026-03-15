//go:build js && !tinygo

package nostr

import ws "github.com/coder/websocket"

// connClose closes the WebSocket connection with the given code and reason.
func connClose(conn *ws.Conn, code ws.StatusCode, reason string) {
	_ = conn.Close(code, reason)
}
