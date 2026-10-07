package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"pipechat/internal/database"
)

// Website groups. People make groups and invite each other on the website, which
// knows names and members but never a group's key. The terminals do the rest:
//
//  1. the owner's terminal creates the group (and its key) and reports the chat id;
//  2. a member who has the key seals it to each new member's public key and uploads it;
//  3. the new member's terminal opens it, joins, and confirms.
//
// The server only ever sees sealed blobs.

// GET /api/sync
func getSync(w http.ResponseWriter, r *http.Request) {
	result, err := database.GetSync(r.Context(), Pool, currentSession(r).userID)
	if err != nil {
		serverError(w, "sync", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// POST /api/sync/groups: the owner's terminal made the group; remember its chat id.
func syncGroupCreated(w http.ResponseWriter, r *http.Request) {
	var request gidRequest
	if !readJSON(w, r, &request) {
		return
	}
	if !groupIDPattern.MatchString(request.GID) {
		writeError(w, http.StatusBadRequest, "bad group id")
		return
	}
	stored, err := database.GetGroupTokenHash(r.Context(), Pool, request.GID)
	if err != nil {
		serverError(w, "get group", err)
		return
	}
	if stored == nil {
		writeError(w, http.StatusNotFound, "create the group first")
		return
	}
	ok, err := database.SetGroupGID(r.Context(), Pool, currentSession(r).userID, request.Group, request.GID)
	if err != nil {
		serverError(w, "set group id", err)
		return
	}
	if !ok {
		writeError(w, http.StatusForbidden, "not your group, or already created")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/sync/grants
func syncGrant(w http.ResponseWriter, r *http.Request) {
	var request grantRequest
	if !readJSON(w, r, &request) {
		return
	}
	if len(request.Sealed) == 0 || len(request.Sealed) > 4<<10 {
		writeError(w, http.StatusBadRequest, "bad sealed key")
		return
	}
	ok, err := database.AddGrant(r.Context(), Pool, currentSession(r).userID, request.Group, request.Handle, request.Sealed)
	if err != nil {
		serverError(w, "add grant", err)
		return
	}
	if !ok {
		writeError(w, http.StatusConflict, "nobody is waiting for that key")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/sync/grants/{group}
func syncGrantReceived(w http.ResponseWriter, r *http.Request) {
	group, err := strconv.ParseInt(chi.URLParam(r, "group"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad group")
		return
	}
	if err := database.AcceptGrant(r.Context(), Pool, currentSession(r).userID, group); err != nil {
		serverError(w, "accept grant", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
