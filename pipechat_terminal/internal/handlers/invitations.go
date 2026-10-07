package handlers

import (
	"fmt"
	"log"
	"mime"
	"net/http"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"pipechat/internal/crypto"
	"pipechat/internal/database"
)

// Invitations. A member asks the server to email a group's invite code to
// someone. The code is the group's encryption key, so the server sees it for
// as long as it takes to send the email; it is never stored or logged. (A member
// can also share the code privately themselves, which keeps it away from the server.)
//
// So the server cannot be used to send spam, the code must belong to a real
// group the asker is in, and everyone is limited to a few invitations an hour.

const (
	invitesPerHour = 10
	emailTimeout   = 20 * time.Second
)

// sendEmail delivers one email. Tests replace it.
var sendEmail = smtp.SendMail

// POST /api/invitations
func inviteByEmail(w http.ResponseWriter, r *http.Request) {
	if Config.SMTPHost == "" || Config.SMTPFrom == "" {
		writeError(w, http.StatusServiceUnavailable, "this server is not set up to send email")
		return
	}
	var request inviteRequest
	if !readJSON(w, r, &request) {
		return
	}
	recipient, err := mail.ParseAddress(request.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, "not an email address")
		return
	}
	secret, err := crypto.ParseCode(request.Code)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid invite code")
		return
	}

	// The code must be a real group's code, and the asker must be in that group.
	stored, err := database.GetGroupTokenHash(r.Context(), Pool, secret.GIDHex())
	if err != nil {
		serverError(w, "get group", err)
		return
	}
	if stored == nil || string(stored) != string(tokenHash(secret.JoinToken())) {
		writeError(w, http.StatusForbidden, "that is not the code of an existing group")
		return
	}
	own := currentSession(r)
	live.mutex.Lock()
	member := own.joined[secret.GIDHex()]
	live.mutex.Unlock()
	if !member {
		writeError(w, http.StatusForbidden, "you are not in that group")
		return
	}

	key := "invite|" + own.handle
	if countRecent(key, time.Hour) >= invitesPerHour {
		writeError(w, http.StatusTooManyRequests, "too many invitations, try again later")
		return
	}
	recordFailure(key)

	if err := emailInvitation(recipient, own.handle, secret); err != nil {
		log.Printf("sending invitation: %v", err)
		writeError(w, http.StatusBadGateway, "could not send the email")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func emailInvitation(recipient *mail.Address, inviter string, secret *crypto.Secret) error {
	sender, err := mail.ParseAddress(Config.SMTPFrom)
	if err != nil {
		return fmt.Errorf("SMTP_FROM is not an email address")
	}
	body := fmt.Sprintf(`@%s invited you to the group "%s" on Pipechat.

Start the chat client, type /join, then paste this code:

%s

Keep this code private: it is the group's encryption key. Anyone who has it can
read and write in the group.
`, inviter, secret.Name, secret.Code())

	headers := []string{
		"From: " + sender.String(),
		"To: " + recipient.String(),
		"Subject: " + mime.QEncoding.Encode("utf-8", `Invitation to join "`+secret.Name+`" on Pipechat`),
		"Date: " + time.Now().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
	}
	message := strings.Join(headers, "\r\n") + "\r\n\r\n" + strings.ReplaceAll(body, "\n", "\r\n")

	var authentication smtp.Auth
	if Config.SMTPUsername != "" {
		authentication = smtp.PlainAuth("", Config.SMTPUsername, Config.SMTPPassword, Config.SMTPHost)
	}

	// smtp.SendMail has no timeout of its own.
	done := make(chan error, 1)
	go func() {
		done <- sendEmail(Config.SMTPHost+":"+Config.SMTPPort, authentication, sender.Address, []string{recipient.Address}, []byte(message))
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(emailTimeout):
		return fmt.Errorf("the mail server did not answer in %s", emailTimeout)
	}
}
