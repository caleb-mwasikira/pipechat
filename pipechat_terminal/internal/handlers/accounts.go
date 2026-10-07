package handlers

import (
	"crypto/rand"
	"log"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"pipechat/internal/crypto"
	"pipechat/internal/database"
)

// Making an account and resetting a password from the terminal. They change the website's own users table
// the way the website does, so there is one account and one password for both: an account made here logs in
// on the website, and a password reset here works there.

// passwordCost is bcrypt's cost, the same as the website's (Laravel's default). Tests make it cheaper.
var passwordCost = 12

const (
	registrationsPerHour = 5
	resetsPerHour        = 3
	resetCodeLife        = 60 * time.Minute // as long as the website lets a reset link live
	resetCodeGap         = time.Minute      // as often as the website sends one
	minPassword          = 8
	maxPassword          = 72 // bcrypt reads no more than this
)

type registerRequest struct {
	Name     string `json:"name"`
	Handle   string `json:"handle"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type forgotRequest struct {
	Email string `json:"email"`
}

type resetRequest struct {
	Email    string `json:"email"`
	Code     string `json:"code"`
	Password string `json:"password"`
}

var plausibleEmail = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// cleanEmail lower-cases and checks an email address; it returns "" when it is not one.
func cleanEmail(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if len(text) > 255 || !plausibleEmail.MatchString(text) {
		return ""
	}
	if parsed, err := mail.ParseAddress(text); err != nil || parsed.Address != text {
		return ""
	}
	return text
}

func checkPassword(password string) string {
	switch {
	case len(password) < minPassword:
		return "the password must be at least 8 characters"
	case len(password) > maxPassword:
		return "the password can be at most 72 characters"
	}
	return ""
}

// POST /api/auth/register
func registerAccount(w http.ResponseWriter, r *http.Request) {
	var request registerRequest
	if !readJSON(w, r, &request) {
		return
	}
	limit := "register|" + clientAddress(r)
	if countRecent(limit, time.Hour) >= registrationsPerHour {
		writeError(w, http.StatusTooManyRequests, "too many accounts made from here; try again later")
		return
	}

	name := strings.TrimSpace(request.Name)
	email := cleanEmail(request.Email)
	switch {
	case name == "" || utf8.RuneCountInString(name) > 100:
		writeError(w, http.StatusBadRequest, "give a name of 1 to 100 characters")
		return
	case !crypto.ValidHandle(request.Handle):
		writeError(w, http.StatusBadRequest, "the handle must be 3-20 characters: a-z, 0-9 or _")
		return
	case email == "":
		writeError(w, http.StatusBadRequest, "that does not look like an email address")
		return
	}
	if problem := checkPassword(request.Password); problem != "" {
		writeError(w, http.StatusBadRequest, problem)
		return
	}
	// A try that passed the checks counts, whether or not it made an account, so the form cannot be used to make
	// accounts in bulk or to find out which emails have one. A typo in a field does not count.
	recordFailure(limit)
	hash, err := bcrypt.GenerateFromPassword([]byte(request.Password), passwordCost)
	if err != nil {
		serverError(w, "hash password", err)
		return
	}
	switch err := database.CreateWebUser(r.Context(), Pool, name, request.Handle, email, string(hash)); err {
	case nil:
	case database.ErrEmailTaken, database.ErrHandleTaken:
		writeError(w, http.StatusConflict, err.Error())
		return
	default:
		serverError(w, "create user", err)
		return
	}

	if mailConfigured() {
		go welcome(name, request.Handle, email)
	}
	writeJSON(w, http.StatusCreated, map[string]string{"handle": request.Handle})
}

// welcome emails a new user. Failing to is only logged: the account exists either way.
func welcome(name string, handle string, email string) {
	recipient := &mail.Address{Name: name, Address: email}
	body := "Welcome to Pipechat, " + name + "!\n\nYour account is ready. Your chat handle is @" + handle + ".\n" +
		"Log in to the terminal client with this email address and your password.\n"
	if Config.WebsiteURL != "" {
		body += "\nYour account also works on the website: " + Config.WebsiteURL + "\n"
	}
	if err := sendMail(recipient, "Welcome to Pipechat", body); err != nil {
		log.Printf("welcome email: %v", err)
	}
}

// resetAlphabet leaves out the characters people mix up: 0 and O, 1 and I.
const resetAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// newResetCode makes a code like "K7QM2XP9RVAH" (about 60 bits, so it cannot be guessed within the attempt limits).
func newResetCode() string {
	raw := make([]byte, 12)
	rand.Read(raw)
	code := make([]byte, 12)
	for index, b := range raw {
		code[index] = resetAlphabet[int(b)%len(resetAlphabet)]
	}
	return string(code)
}

// normalCode accepts a code typed with dashes, spaces or small letters.
func normalCode(text string) string {
	return strings.ToUpper(strings.Map(func(r rune) rune {
		if r == '-' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, text))
}

func spacedCode(code string) string {
	return code[0:4] + "-" + code[4:8] + "-" + code[8:12]
}

// dummyHash makes unknown emails cost the same work as known ones.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("no such code"), passwordCost)

// POST /api/auth/forgot: email a reset code. The answer is the same whether or not the email has an account,
// and comes before any of the work, so it cannot be used to find out who has one.
func forgotPassword(w http.ResponseWriter, r *http.Request) {
	var request forgotRequest
	if !readJSON(w, r, &request) {
		return
	}
	if !mailConfigured() {
		writeError(w, http.StatusServiceUnavailable, "this server cannot send email, so it cannot reset a password; use the website")
		return
	}
	email := cleanEmail(request.Email)
	if email == "" {
		writeError(w, http.StatusBadRequest, "that does not look like an email address")
		return
	}
	address, byEmail := "forgot|"+clientAddress(r), "forgot|"+email
	if countRecent(address, time.Hour) >= resetsPerHour*3 || countRecent(byEmail, time.Hour) >= resetsPerHour {
		writeError(w, http.StatusTooManyRequests, "too many requests; try again in a while")
		return
	}
	recordFailure(address)
	recordFailure(byEmail)

	go sendResetCode(email)
	writeJSON(w, http.StatusAccepted, map[string]string{"message": "If that email has an account, a code is on its way."})
}

// sendResetCode makes and emails a code, if the email belongs to a user and none was sent a moment ago.
func sendResetCode(email string) {
	user, err := database.GetWebUser(backgroundContext(), Pool, email)
	code := newResetCode()
	hash, hashErr := bcrypt.GenerateFromPassword([]byte(code), passwordCost) // the work is done even when there is no user
	if err != nil || hashErr != nil || user == nil {
		return
	}
	stored, err := database.StoreResetToken(backgroundContext(), Pool, email, string(hash), resetCodeGap)
	if err != nil {
		log.Printf("store reset code: %v", err)
		return
	}
	if !stored {
		return // one was sent a moment ago; the website holds back the same way
	}
	body := "We got a request to reset the password of your Pipechat account.\n\n" +
		"Your code is:\n\n    " + spacedCode(code) + "\n\n" +
		"Type it into the terminal client, with your new password. It works for 60 minutes.\n"
	if Config.WebsiteURL != "" {
		link := Config.WebsiteURL + "/reset-password/" + code + "?email=" + url.QueryEscape(email)
		body += "\nOr choose a new password on the website:\n" + link + "\n"
	}
	body += "\nIf you did not ask for this, ignore this email: your password stays the same.\n" +
		"After a reset, the terminal starts with new keys, and your groups come back as other members come online.\n"
	if err := sendMail(&mail.Address{Address: email}, "Your Pipechat password reset code", body); err != nil {
		log.Printf("reset email: %v", err)
	}
}

// POST /api/auth/reset: use a code to set a new password.
func resetPassword(w http.ResponseWriter, r *http.Request) {
	var request resetRequest
	if !readJSON(w, r, &request) {
		return
	}
	email, code := cleanEmail(request.Email), normalCode(request.Code)
	key := "reset|" + clientAddress(r) + "|" + email
	if tooManyFailures(key) {
		writeError(w, http.StatusTooManyRequests, "too many wrong codes; try again later")
		return
	}
	if email == "" || len(code) != 12 {
		writeError(w, http.StatusBadRequest, "enter the email and the 12-character code from the email")
		return
	}
	if problem := checkPassword(request.Password); problem != "" {
		writeError(w, http.StatusBadRequest, problem)
		return
	}

	hash, age, found, err := database.GetResetToken(r.Context(), Pool, email)
	if err != nil {
		serverError(w, "get reset code", err)
		return
	}
	stored := dummyHash // compared even when there is no code, so the time taken tells nothing
	if found {
		stored = []byte(hash)
	}
	// Laravel writes $2y$ and Go reads the same thing as $2a$.
	matches := bcrypt.CompareHashAndPassword([]byte(strings.Replace(string(stored), "$2y$", "$2a$", 1)), []byte(code)) == nil
	if !found || !matches || age > resetCodeLife {
		recordFailure(key)
		writeError(w, http.StatusBadRequest, "that code is wrong or has expired")
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(request.Password), passwordCost)
	if err != nil {
		serverError(w, "hash password", err)
		return
	}
	if err := database.ResetPassword(r.Context(), Pool, email, string(newHash)); err != nil {
		serverError(w, "reset password", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
