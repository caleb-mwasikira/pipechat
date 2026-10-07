package handlers

import (
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"

	"pipechat/internal/crypto"
	"pipechat/internal/database"
)

// Open groups: a creator lists a group under a name, and anyone who types /join NAME gets in.
// The server holds the invite code to hand over, so these groups are public by design.

var openNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{3,24}$`)

const (
	openGroupsListed = 200
	openGroupsLimit  = 5000
)

type openRequest struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

// POST /api/open: the creator makes a group they are in public.
func listOpenGroup(w http.ResponseWriter, r *http.Request) {
	var request openRequest
	if !readJSON(w, r, &request) {
		return
	}
	if !openNamePattern.MatchString(request.Name) {
		writeError(w, http.StatusBadRequest, "a public group's name is 3-24 characters: letters, digits, - or _")
		return
	}
	secret, err := crypto.ParseCode(request.Code)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid invite code")
		return
	}
	// The code must be the real key of a group the asker is in, so nobody can list someone else's group.
	stored, err := database.GetGroupTokenHash(r.Context(), Pool, secret.GIDHex())
	if err != nil {
		serverError(w, "get group", err)
		return
	}
	own := currentSession(r)
	live.mutex.Lock()
	member := own.joined[secret.GIDHex()]
	live.mutex.Unlock()
	if stored == nil || string(stored) != string(tokenHash(secret.JoinToken())) || !member {
		writeError(w, http.StatusForbidden, "you are not in that group")
		return
	}
	existing, err := database.ListOpenGroups(r.Context(), Pool, openGroupsLimit+1)
	if err != nil {
		serverError(w, "list open groups", err)
		return
	}
	if len(existing) > openGroupsLimit {
		writeError(w, http.StatusForbidden, "too many public groups")
		return
	}
	created, err := database.AddOpenGroup(r.Context(), Pool, request.Name, secret.GIDHex(), secret.Code(), own.handle)
	if err != nil {
		serverError(w, "add open group", err)
		return
	}
	if !created {
		writeError(w, http.StatusConflict, "that public name is taken, or the group is already public")
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// GET /api/open
func listOpenGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := database.ListOpenGroups(r.Context(), Pool, openGroupsListed)
	if err != nil {
		serverError(w, "list open groups", err)
		return
	}
	writeJSON(w, http.StatusOK, groups)
}

// GET /api/open/{name}: what a person needs to join.
func getOpenGroup(w http.ResponseWriter, r *http.Request) {
	group, err := database.GetOpenGroup(r.Context(), Pool, chi.URLParam(r, "name"))
	if err != nil {
		serverError(w, "get open group", err)
		return
	}
	if group == nil {
		writeError(w, http.StatusNotFound, "no public group with that name")
		return
	}
	writeJSON(w, http.StatusOK, group)
}

// DELETE /api/open/{name}: the creator makes it private again. People already in stay in.
func unlistOpenGroup(w http.ResponseWriter, r *http.Request) {
	removed, err := database.RemoveOpenGroup(r.Context(), Pool, chi.URLParam(r, "name"), currentSession(r).handle)
	if err != nil {
		serverError(w, "remove open group", err)
		return
	}
	if !removed {
		writeError(w, http.StatusNotFound, "no public group of yours with that name")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
