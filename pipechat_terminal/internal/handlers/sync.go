package handlers

import (
	"crypto/ecdh"
	"encoding/hex"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"pipechat/internal/crypto"
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

// serverFrom is the name a group key comes from when the server itself hands it over.
const serverFrom = "pipechat"

// escrowIdentity is the server's own key pair, or nil when CHAT_ESCROW_KEY is not set.
func escrowIdentity() *ecdh.PrivateKey {
	if len(Config.EscrowKey) == 0 {
		return nil
	}
	identity, err := crypto.EscrowIdentity(Config.EscrowKey)
	if err != nil {
		return nil
	}
	return identity
}

// GET /api/sync
func getSync(w http.ResponseWriter, r *http.Request) {
	own := currentSession(r)
	result, err := database.GetSync(r.Context(), Pool, own.userID)
	if err != nil {
		serverError(w, "sync", err)
		return
	}
	result.NeedEscrow = orEmpty(result.NeedEscrow)

	// With escrow on, the server hands waiting members their group keys itself, sealed to
	// the member's own public key, so nobody else has to be online.
	if server := escrowIdentity(); server != nil {
		serverKey := hex.EncodeToString(server.PublicKey().Bytes())
		result.ServerKey = serverKey
		account, err := database.GetAccount(r.Context(), Pool, own.handle)
		if err != nil {
			serverError(w, "get account", err)
			return
		}
		if memberKey, err := crypto.ParsePublicKey(accountKey(account)); err == nil {
			already := map[int64]bool{}
			for _, item := range result.Incoming {
				already[item.Group] = true
			}
			for _, item := range result.Escrowed {
				secret, err := crypto.OpenAnon(server, item.Escrow)
				if err != nil || already[item.Group] {
					continue
				}
				sealed, err := crypto.SealFor(server, memberKey, secret)
				if err != nil {
					continue
				}
				result.Incoming = append(result.Incoming, database.Incoming{
					Group: item.Group, Name: item.Name, From: serverFrom, FromKey: serverKey, Sealed: sealed,
				})
			}
		}
	} else {
		result.NeedEscrow = []database.MissingEscrow{}
	}
	writeJSON(w, http.StatusOK, result)
}

func accountKey(account *database.Account) string {
	if account == nil {
		return ""
	}
	return account.PubKey
}

func orEmpty[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}

// POST /api/sync/escrow: a member who has a group's key lets the server hold a copy, sealed so
// only the server can open it. The server checks it really is that group's key.
func syncEscrow(w http.ResponseWriter, r *http.Request) {
	server := escrowIdentity()
	if server == nil {
		writeError(w, http.StatusNotFound, "this server does not hold group keys")
		return
	}
	var request escrowRequest
	if !readJSON(w, r, &request) {
		return
	}
	secret, err := crypto.OpenAnon(server, request.Sealed)
	gid, gidErr := database.GroupGID(r.Context(), Pool, request.Group)
	if err != nil || gidErr != nil || gid == "" || secret.GIDHex() != gid {
		writeError(w, http.StatusBadRequest, "that is not this group's key")
		return
	}
	stored, err := database.GetGroupTokenHash(r.Context(), Pool, gid)
	if err != nil || stored == nil || string(stored) != string(tokenHash(secret.JoinToken())) {
		writeError(w, http.StatusBadRequest, "that is not this group's key")
		return
	}
	ok, err := database.SetEscrow(r.Context(), Pool, currentSession(r).userID, request.Group, gid, request.Sealed)
	if err != nil {
		serverError(w, "set escrow", err)
		return
	}
	if !ok {
		writeError(w, http.StatusConflict, "already held, or you do not have this group's key")
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
