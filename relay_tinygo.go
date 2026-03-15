//go:build tinygo && js

package nostr

import ws "github.com/coder/websocket"

// connClose closes the WebSocket connection.
// In TinyGo, conn.Close() can panic via syscall/js when the JS runtime
// throws an exception with a non-standard error type that can't be recovered.
// We skip the close handshake and let the JS runtime clean up the connection.
func connClose(conn *ws.Conn, code ws.StatusCode, reason string) {
}
