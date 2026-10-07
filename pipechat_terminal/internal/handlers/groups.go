package handlers

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"regexp"
	"sort"

	"github.com/go-chi/chi/v5"

	"pipechat/internal/database"
)

// Groups. The server knows a group's public id, the SHA-256 of a join token
// derived from the group secret (so it can tell members from strangers), and
// ciphertext. It never sees the secret itself.

var groupIDPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

func tokenHash(token string) []byte {
	raw, _ := hex.DecodeString(token)
	sum := sha256.Sum256(raw)
	return sum[:]
}

func toEvent(groupID string, message database.Message) event {
	return event{Type: "msg", GID: groupID, CT: message.CT, Seq: message.Seq, TS: message.TS}
}

// POST /api/groups: register a new group and subscribe to it.
func createGroup(w http.ResponseWriter, r *http.Request) {
	var request createGroupRequest
	if !readJSON(w, r, &request) {
		return
	}
	if !groupIDPattern.MatchString(request.GID) || !hex64.MatchString(request.Tok) {
		writeError(w, http.StatusBadRequest, "bad group id or token")
		return
	}
	own := currentSession(r)

	count, err := database.CountGroups(r.Context(), Pool)
	if err != nil {
		serverError(w, "count groups", err)
		return
	}
	live.mutex.Lock()
	tooMany := len(own.joined) >= groupsPerSessionLimit
	live.mutex.Unlock()
	if count >= groupsLimit || tooMany {
		writeError(w, http.StatusForbidden, "too many groups")
		return
	}

	created, err := database.CreateGroup(r.Context(), Pool, request.GID, tokenHash(request.Tok))
	if err != nil {
		serverError(w, "create group", err)
		return
	}
	if !created {
		writeError(w, http.StatusConflict, "group already exists")
		return
	}
	live.mutex.Lock()
	subscribe(own, request.GID)
	live.mutex.Unlock()
	w.WriteHeader(http.StatusCreated)
}

// subscribe must be called with live.mutex held.
func subscribe(entry *session, groupID string) {
	if live.members[groupID] == nil {
		live.members[groupID] = map[*session]bool{}
	}
	live.members[groupID][entry] = true
	entry.joined[groupID] = true
}

// POST /api/groups/{gid}/join: prove knowledge of the join token, get the messages
// missed since a sequence number, and subscribe to new ones on /api/events.
func joinGroup(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "gid")
	var request joinRequest
	if !readJSON(w, r, &request) {
		return
	}
	own := currentSession(r)
	key := "join|" + own.handle
	if tooManyFailures(key) {
		writeError(w, http.StatusTooManyRequests, "too many wrong codes, try again later")
		return
	}
	if !groupIDPattern.MatchString(groupID) || !hex64.MatchString(request.Tok) {
		writeError(w, http.StatusBadRequest, "bad group id or token")
		return
	}

	stored, err := database.GetGroupTokenHash(r.Context(), Pool, groupID)
	if err != nil {
		serverError(w, "get group", err)
		return
	}
	// The same answer for an unknown group and a wrong token, so ids cannot be probed.
	if stored == nil || subtle.ConstantTimeCompare(stored, tokenHash(request.Tok)) != 1 {
		recordFailure(key)
		writeError(w, http.StatusForbidden, "no such group, or wrong code")
		return
	}

	// A website group is only for its members: knowing the key is not enough once someone was removed.
	if website, member, err := database.WebGroupMember(r.Context(), Pool, groupID, own.userID); err != nil {
		serverError(w, "check membership", err)
		return
	} else if website && !member {
		writeError(w, http.StatusForbidden, "you are no longer a member of this group")
		return
	}

	// History and subscription happen under one lock, so no message is missed or sent twice.
	live.mutex.Lock()
	defer live.mutex.Unlock()
	if !own.joined[groupID] && len(own.joined) >= groupsPerSessionLimit {
		writeError(w, http.StatusForbidden, "too many groups")
		return
	}
	missed, err := database.MessagesSince(r.Context(), Pool, groupID, request.Since, historySent)
	if err != nil {
		serverError(w, "get messages", err)
		return
	}
	subscribe(own, groupID)

	response := joinResponse{Messages: make([]event, 0, len(missed))}
	for _, message := range missed {
		response.Messages = append(response.Messages, toEvent(groupID, message))
	}
	writeJSON(w, http.StatusOK, response)
}

// DELETE /api/groups/{gid}/membership
func leaveGroup(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "gid")
	own := currentSession(r)

	live.mutex.Lock()
	delete(live.members[groupID], own)
	delete(own.joined, groupID)
	live.mutex.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/groups/{gid}/messages: store ciphertext and hand it to everyone
// subscribed, the sender included.
func sendMessage(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "gid")
	var request sendRequest
	if !readJSON(w, r, &request) {
		return
	}
	if len(request.CT) == 0 || len(request.CT) > 16<<10 {
		writeError(w, http.StatusBadRequest, "message empty or too large")
		return
	}
	own := currentSession(r)

	// Saving and sending under one lock keeps every subscriber's order the same as the sequence numbers.
	live.mutex.Lock()
	defer live.mutex.Unlock()
	if !own.joined[groupID] {
		writeError(w, http.StatusForbidden, "not in this group")
		return
	}
	message, err := database.AppendMessage(r.Context(), Pool, groupID, request.CT, historyKept)
	if err != nil {
		serverError(w, "save message", err)
		return
	}
	for member := range live.members[groupID] {
		deliver(member, toEvent(groupID, message))
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/groups/{gid}/online: who has this chat open right now. Only people in the group may ask.
func groupOnline(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "gid")
	own := currentSession(r)

	live.mutex.Lock()
	defer live.mutex.Unlock()
	if !own.joined[groupID] {
		writeError(w, http.StatusForbidden, "not in this group")
		return
	}
	seen := map[string]bool{}
	handles := []string{}
	for member := range live.members[groupID] {
		if member.socket && !seen[member.handle] {
			seen[member.handle] = true
			handles = append(handles, member.handle)
		}
	}
	sort.Strings(handles)
	writeJSON(w, http.StatusOK, handles)
}
