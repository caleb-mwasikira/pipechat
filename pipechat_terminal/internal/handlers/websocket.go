package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"pipechat/internal/database"
)

const (
	webSocketPing    = 20 * time.Second
	webSocketTimeout = 10 * time.Second
)

// GET /api/ws upgrades to a WebSocket that carries everything for the logged-in
// session: messages of the groups it joined ("msg") and direct-chat notices
// ("dm"), one JSON event per text message. The client only listens; it sends
// with the POST requests. A newer socket for the same session replaces the older one.
func chatSocket(w http.ResponseWriter, r *http.Request) {
	own := currentSession(r)

	connection, err := websocket.Accept(w, r, nil)
	if err != nil {
		return // Accept has already answered
	}
	defer connection.CloseNow()

	// CloseRead answers pings and close frames, and cancels ctx when the client goes away.
	ctx := connection.CloseRead(r.Context())

	live.mutex.Lock()
	if own.stream != nil {
		close(own.stream)
	}
	stop := make(chan struct{})
	own.stream = stop
	own.socket = true
	live.mutex.Unlock()
	database.SetPresence(ctx, Pool, own.handle, true)
	defer func() {
		live.mutex.Lock()
		if own.stream == stop { // not already replaced by a newer socket
			own.stream, own.socket = nil, false
		}
		stillOnline := false
		for other := range live.online[own.handle] {
			stillOnline = stillOnline || other.socket
		}
		live.mutex.Unlock()
		if !stillOnline {
			database.SetPresence(context.Background(), Pool, own.handle, false)
		}
	}()

	ping := time.NewTicker(webSocketPing)
	defer ping.Stop()
	for {
		select {
		case message := <-own.events:
			writeContext, cancel := context.WithTimeout(ctx, webSocketTimeout)
			err := wsjson.Write(writeContext, connection, message)
			cancel()
			if err != nil {
				return
			}
		case <-ping.C:
			live.mutex.Lock()
			own.lastSeen = time.Now()
			live.mutex.Unlock()
			database.SetPresence(ctx, Pool, own.handle, true) // so the website knows we are still here
			pingContext, cancel := context.WithTimeout(ctx, webSocketTimeout)
			err := connection.Ping(pingContext)
			cancel()
			if err != nil {
				return
			}
		case <-stop:
			connection.Close(websocket.StatusNormalClosure, "replaced by a newer connection")
			return
		case <-own.closed:
			connection.Close(websocket.StatusPolicyViolation, "session ended, log in again")
			return
		case <-ctx.Done():
			return
		}
	}
}
