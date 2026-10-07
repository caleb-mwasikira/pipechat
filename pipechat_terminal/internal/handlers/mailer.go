package handlers

import (
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// smtpSend is how mail is sent. Port 465 speaks TLS from the first byte (smtps), which smtp.SendMail does not;
// every other port starts plain and upgrades when the server offers it, as smtp.SendMail does.
var smtpSend = func(address string, authentication smtp.Auth, from string, to []string, message []byte) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port != "465" {
		return smtp.SendMail(address, authentication, from, to, message)
	}
	connection, err := tls.Dial("tcp", address, &tls.Config{ServerName: host})
	if err != nil {
		return err
	}
	client, err := smtp.NewClient(connection, host)
	if err != nil {
		connection.Close()
		return err
	}
	defer client.Close()
	if authentication != nil {
		if err := client.Auth(authentication); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(message); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

// mailConfigured says whether this server can send email at all.
func mailConfigured() bool { return Config.SMTPHost != "" && Config.SMTPFrom != "" }

// sendMail sends one plain text email, giving up after emailTimeout (smtp.SendMail has no timeout of its own).
func sendMail(recipient *mail.Address, subject string, body string) error {
	sender, err := mail.ParseAddress(Config.SMTPFrom)
	if err != nil {
		return fmt.Errorf("SMTP_FROM is not an email address")
	}
	headers := []string{
		"From: " + sender.String(),
		"To: " + recipient.String(),
		"Subject: " + mime.QEncoding.Encode("utf-8", subject),
		"Date: " + time.Now().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
	}
	message := strings.Join(headers, "\r\n") + "\r\n\r\n" + strings.ReplaceAll(body, "\n", "\r\n")

	var authentication smtp.Auth
	if Config.SMTPUsername != "" {
		authentication = smtp.PlainAuth("", Config.SMTPUsername, Config.SMTPPassword, Config.SMTPHost)
	}
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

// SetMailSender replaces how email is sent, and returns what puts it back. It is for tests of programs that use
// this server, which need to read the email it sends.
func SetMailSender(send func(address string, authentication smtp.Auth, from string, to []string, message []byte) error) (restore func()) {
	previous := sendEmail
	sendEmail = send
	return func() { sendEmail = previous }
}
