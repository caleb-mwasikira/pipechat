package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"pipechat/internal/crypto"
	"pipechat/internal/database"
)

// Accounts. A chat account belongs to a website user and logs in with the same
// email and password. It is a handle, an X25519 identity public key and a "vault":
// a blob sealed with a key derived from the password, which the server cannot open.

type contextKey string

// A handler behind requireLogin reads the logged-in session with:
//
//	user := r.Context().Value(sessionContextKey).(*session)
const sessionContextKey contextKey = "session"

var hex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

const (
	failuresLimit  = 10
	failuresWindow = 10 * time.Minute
)

// failures counts bad logins and bad group codes per key, so codes and passwords cannot be guessed.
var failures = struct {
	mutex  sync.Mutex
	counts map[string][]time.Time
}{counts: map[string][]time.Time{}}

// countRecent is how many times key was recorded within the window.
func countRecent(key string, window time.Duration) int {
	failures.mutex.Lock()
	defer failures.mutex.Unlock()
	recent := failures.counts[key][:0]
	for _, when := range failures.counts[key] {
		if time.Since(when) < window {
			recent = append(recent, when)
		}
	}
	failures.counts[key] = recent
	return len(recent)
}

func tooManyFailures(key string) bool {
	return countRecent(key, failuresWindow) >= failuresLimit
}

func recordFailure(key string) {
	failures.mutex.Lock()
	defer failures.mutex.Unlock()
	failures.counts[key] = append(failures.counts[key], time.Now())
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

func serverError(w http.ResponseWriter, what string, err error) {
	log.Printf("%s: %v", what, err)
	writeError(w, http.StatusInternalServerError, "server error")
}

func readJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 96<<10)
	if err := json.NewDecoder(r.Body).Decode(into); err != nil {
		writeError(w, http.StatusBadRequest, "malformed request")
		return false
	}
	return true
}

// requireLogin rejects requests without a valid "Authorization: Bearer <token>" and
// puts the session in the request context.
func requireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")

		live.mutex.Lock()
		entry := live.sessions[token]
		if entry != nil {
			entry.lastSeen = time.Now()
		}
		live.mutex.Unlock()

		if entry == nil {
			writeError(w, http.StatusUnauthorized, "log in again")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionContextKey, entry)))
	})
}

func currentSession(r *http.Request) *session {
	return r.Context().Value(sessionContextKey).(*session)
}

func clientAddress(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// A bcrypt hash that matches nothing, checked when the email is unknown so that
// unknown emails and wrong passwords take the same time.
var noUserHash, _ = bcrypt.GenerateFromPassword([]byte("no such user"), bcrypt.DefaultCost)

// POST /api/auth/login: the same email and password as on the website. The first
// login creates the user's chat account from their website handle; the terminal
// then sends its new keys with PUT /api/identity.
func login(w http.ResponseWriter, r *http.Request) {
	var request authRequest
	if !readJSON(w, r, &request) {
		return
	}
	key := "login|" + clientAddress(r) + "|" + strings.ToLower(request.Email)
	if tooManyFailures(key) {
		writeError(w, http.StatusTooManyRequests, "too many failed logins, try again later")
		return
	}

	user, err := database.GetWebUser(r.Context(), Pool, request.Email)
	if err != nil {
		serverError(w, "get user", err)
		return
	}
	hash := noUserHash
	if user != nil {
		// Laravel writes bcrypt hashes with the $2y$ prefix; Go knows the same thing as $2a$.
		hash = []byte(strings.Replace(user.PasswordHash, "$2y$", "$2a$", 1))
	}
	if bcrypt.CompareHashAndPassword(hash, []byte(request.Password)) != nil || user == nil {
		recordFailure(key)
		writeError(w, http.StatusUnauthorized, "wrong email or password")
		return
	}
	if !crypto.ValidHandle(user.Handle) {
		writeError(w, http.StatusForbidden, "this account has no chat handle yet")
		return
	}

	account, err := database.AccountForUser(r.Context(), Pool, user)
	if err == nil && account == nil {
		count, countErr := database.CountAccounts(r.Context(), Pool)
		if countErr != nil {
			serverError(w, "count accounts", countErr)
			return
		}
		if count >= accountsLimit {
			writeError(w, http.StatusServiceUnavailable, "server is full")
			return
		}
		var created bool
		if created, err = database.LinkAccount(r.Context(), Pool, user); err == nil {
			if !created {
				writeError(w, http.StatusConflict, "the handle @"+user.Handle+" belongs to another chat account")
				return
			}
			account, err = database.AccountForUser(r.Context(), Pool, user)
		}
	}
	if err != nil || account == nil {
		serverError(w, "get account", err)
		return
	}
	peers, err := database.GetInbox(r.Context(), Pool, account.Handle)
	if err != nil {
		serverError(w, "get inbox", err)
		return
	}

	live.mutex.Lock()
	entry := startSession(account.Handle)
	entry.userID = user.ID
	live.mutex.Unlock()
	writeJSON(w, http.StatusOK, authResponse{Token: entry.token, Handle: account.Handle, Vault: account.Vault, PubKey: account.PubKey, Peers: peers})
}

// PUT /api/identity: the terminal's new public key and the vault sealed with it, on
// the first login or when the old vault could not be opened (after a password reset).
func saveIdentity(w http.ResponseWriter, r *http.Request) {
	var request identityRequest
	if !readJSON(w, r, &request) {
		return
	}
	if !hex64.MatchString(request.PubKey) || len(request.Vault) == 0 || len(request.Vault) > 32<<10 {
		writeError(w, http.StatusBadRequest, "bad key or vault")
		return
	}
	if err := database.SetIdentity(r.Context(), Pool, currentSession(r).handle, request.PubKey, request.Vault); err != nil {
		serverError(w, "save identity", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PUT /api/vault
func saveVault(w http.ResponseWriter, r *http.Request) {
	var request vaultRequest
	if !readJSON(w, r, &request) {
		return
	}
	if len(request.Vault) == 0 || len(request.Vault) > 32<<10 {
		writeError(w, http.StatusBadRequest, "vault empty or too large")
		return
	}
	if err := database.SetVault(r.Context(), Pool, currentSession(r).handle, request.Vault); err != nil {
		serverError(w, "save vault", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/users/{handle}
func lookupUser(w http.ResponseWriter, r *http.Request) {
	handle := chi.URLParam(r, "handle")
	account, err := database.GetAccount(r.Context(), Pool, handle)
	if err != nil {
		serverError(w, "get account", err)
		return
	}
	if account == nil || account.PubKey == "" {
		writeError(w, http.StatusNotFound, "no user @"+handle)
		return
	}
	writeJSON(w, http.StatusOK, userResponse{Handle: handle, PubKey: account.PubKey})
}

// POST /api/direct/{handle}: "I opened a direct chat with you". The notice is kept
// until the other person clears it, so it reaches them even when they are offline.
func notifyDirect(w http.ResponseWriter, r *http.Request) {
	own := currentSession(r)
	target := chi.URLParam(r, "handle")
	account, err := database.GetAccount(r.Context(), Pool, target)
	if err != nil {
		serverError(w, "get account", err)
		return
	}
	if account == nil || target == own.handle {
		writeError(w, http.StatusNotFound, "no such user")
		return
	}
	if err := database.AddInbox(r.Context(), Pool, target, own.handle, inboxLimit); err != nil {
		serverError(w, "add inbox", err)
		return
	}
	live.mutex.Lock()
	for other := range live.online[target] {
		deliver(other, event{Type: "dm", Handle: own.handle})
	}
	live.mutex.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/direct/{handle}
func clearDirect(w http.ResponseWriter, r *http.Request) {
	if err := database.ClearInbox(r.Context(), Pool, currentSession(r).handle, chi.URLParam(r, "handle")); err != nil {
		serverError(w, "clear inbox", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
