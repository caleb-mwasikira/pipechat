package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"sync"
	"time"

	"pipechat/internal/config"
)

// Pool and Config are set by cmd/server before Routes is used.
var (
	Pool   *sql.DB
	Config *config.Config
)

const (
	historyKept           = 1000 // messages kept per group
	historySent           = 500  // most messages sent to someone joining
	groupsLimit           = 10000
	groupsPerSessionLimit = 50
	accountsLimit         = 10000
	inboxLimit            = 200
	eventQueue            = 512
	sessionIdle           = 24 * time.Hour
)

// A session is one logged-in client. Sessions live in memory only: after a
// restart clients get 401, log in again and join their groups again.
type session struct {
	token    string
	handle   string
	userID   int64 // the website user
	events   chan event
	joined   map[string]bool
	lastSeen time.Time
	closed   chan struct{} // closed when the session ends
	stream   chan struct{} // closed to stop the current event stream when a new one starts
}

// live lists who is connected. Its mutex guards everything in it and the sessions' fields.
var live = struct {
	mutex    sync.Mutex
	sessions map[string]*session
	members  map[string]map[*session]bool // group id -> sessions subscribed to it
	online   map[string]map[*session]bool // handle -> its sessions
}{
	sessions: map[string]*session{},
	members:  map[string]map[*session]bool{},
	online:   map[string]map[*session]bool{},
}

func randomHex(size int) string {
	raw := make([]byte, size)
	rand.Read(raw)
	return hex.EncodeToString(raw)
}

// startSession must be called with live.mutex held.
func startSession(handle string) *session {
	entry := &session{
		token:    randomHex(32),
		handle:   handle,
		events:   make(chan event, eventQueue),
		joined:   map[string]bool{},
		lastSeen: time.Now(),
		closed:   make(chan struct{}),
	}
	live.sessions[entry.token] = entry
	if live.online[handle] == nil {
		live.online[handle] = map[*session]bool{}
	}
	live.online[handle][entry] = true
	return entry
}

// endSession must be called with live.mutex held.
func endSession(entry *session) {
	if live.sessions[entry.token] != entry {
		return
	}
	delete(live.sessions, entry.token)
	delete(live.online[entry.handle], entry)
	for id := range entry.joined {
		delete(live.members[id], entry)
	}
	close(entry.closed)
}

// deliver queues an event. A client too slow to read is dropped; it logs in and
// asks for what it missed (join with since), so nothing is lost. Hold live.mutex.
func deliver(entry *session, message event) {
	select {
	case entry.events <- message:
	default:
		endSession(entry)
	}
}

// SweepSessions ends sessions that have been silent for a day. Run it in a goroutine.
func SweepSessions() {
	for {
		time.Sleep(time.Minute)
		live.mutex.Lock()
		for _, entry := range live.sessions {
			if time.Since(entry.lastSeen) > sessionIdle {
				endSession(entry)
			}
		}
		live.mutex.Unlock()
	}
}
