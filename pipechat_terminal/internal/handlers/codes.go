package handlers

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"pipechat/internal/database"
)

var inviteCodePattern = regexp.MustCompile(`^[A-HJ-NP-Z2-9]{8}$`)

// POST /api/join/code: redeem a short invite code made on the website. The person becomes a member;
// their client then syncs and receives the group key.
func joinWithCode(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Code string `json:"code"`
	}
	if !readJSON(w, r, &request) {
		return
	}
	own := currentSession(r)
	key := "code|" + own.handle
	if tooManyFailures(key) {
		writeError(w, http.StatusTooManyRequests, "too many wrong codes, try again later")
		return
	}
	code := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(request.Code))
	if !inviteCodePattern.MatchString(code) {
		recordFailure(key)
		writeError(w, http.StatusNotFound, "no such invite code")
		return
	}
	name, added, err := database.JoinByCode(r.Context(), Pool, own.userID, code)
	switch {
	case errors.Is(err, database.ErrNoSuchCode):
		recordFailure(key)
		writeError(w, http.StatusNotFound, "no such invite code, or it has expired")
	case errors.Is(err, database.ErrTooManyGroups):
		writeError(w, http.StatusForbidden, "you are in too many groups to join another")
	case err != nil:
		serverError(w, "join with code", err)
	default:
		writeJSON(w, http.StatusOK, map[string]any{"name": name, "added": added})
	}
}
