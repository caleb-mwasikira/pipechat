package client

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"

	"pipechat/internal/client/ui"
	"pipechat/internal/crypto"
)

// The screens before logging in: making an account, asking for a password reset code, and choosing a new
// password with it. They talk to the same account the website has, so an account made here logs in on the
// website too, and a password reset here works there.

type authMode string

const (
	modeLogin    authMode = ""
	modeRegister authMode = "register"
	modeForgot   authMode = "forgot"
	modeReset    authMode = "reset"
)

// Results of the requests.
type registerResult struct {
	email    string
	password string
	err      error
}
type forgotResult struct {
	email string
	err   error
}
type resetResult struct {
	email string
	err   error
}

// goToAuth switches to one of the screens and starts it clean: passwords and codes are never carried over.
func (m *Model) goToAuth(mode authMode) {
	email := strings.TrimSpace(m.emailInput.Value())
	m.authMode, m.authError, m.authNotice, m.authBusy = mode, "", "", false
	for _, field := range []*textinput.Model{&m.passwordInput, &m.confirmInput, &m.codeInput} {
		field.SetValue("")
	}
	if mode != modeLogin {
		m.emailInput.SetValue(email)
	}
	if mode == modeLogin && email == "" {
		m.emailInput.SetValue(m.store.LastEmail)
	}
	m.focusField(0)
	// A screen that starts with the email already filled in begins on the first empty field.
	for position, field := range m.fields() {
		if field.Value() == "" {
			m.focusField(position)
			break
		}
	}
}

// authLabels names the fields of the screen that is showing, in order.
func (m *Model) authLabels() []string {
	switch m.authMode {
	case modeRegister:
		return []string{"Your name", "Handle (what others see: a-z, 0-9, _)", "Email", "Password (8 or more characters)", "Repeat the password"}
	case modeForgot:
		return []string{"The email you registered with"}
	case modeReset:
		return []string{"Email", "Code from the email", "New password (8 or more characters)", "Repeat the new password"}
	}
	return []string{"Email", "Password"}
}

// authLines is everything inside the box of the login, register and reset screens.
func (m *Model) authLines() []string {
	title, hint := "Log in to Pipechat", []string{"Enter to log in · Ctrl+R create an account · Ctrl+F forgot your password", "Ctrl+S change server · Ctrl+C quit"}
	switch m.authMode {
	case modeRegister:
		title, hint = "Create your account", []string{"Enter to go on, and on the last field to create it", "Esc back to log in"}
	case modeForgot:
		title, hint = "Reset your password", []string{"Enter to send a code to that email", "Ctrl+T I already have a code · Esc back to log in"}
	case modeReset:
		title, hint = "Choose a new password", []string{"Enter to set it and go back to log in", "Esc back to log in"}
	}

	lines := []string{ui.Title.Render(title), ""}
	if m.authMode == modeForgot {
		lines = append(lines, ui.Dim.Render("We will email you a code. Type it on the next screen with a new password."), "")
	}
	for index, field := range m.fields() {
		lines = append(lines, m.authLabels()[index], field.View(), "")
	}
	if m.authMode == modeReset {
		lines = append(lines, ui.Dim.Render("Your saved keys are protected by your password. After a reset you start with new keys;"),
			ui.Dim.Render("your groups come back as other members come online."), "")
	}

	switch {
	case m.authBusy || m.pending != nil:
		lines = append(lines, ui.Dim.Render("working..."))
	case m.authError != "":
		lines = append(lines, ui.Error.Render(m.authError))
	case m.authNotice != "":
		lines = append(lines, ui.Brand.Render(m.authNotice))
	case !m.online:
		lines = append(lines, ui.Dim.Render("connecting to "+m.server.URL()+"..."))
	default:
		lines = append(lines, ui.Dim.Render("connected to "+m.server.URL()))
	}
	for _, line := range hint {
		lines = append(lines, ui.Dim.Render(line))
	}
	if m.server != nil && m.authMode == modeLogin {
		// The website is the other place to do all of this.
		lines = append(lines, "",
			ui.Dim.Render("Prefer a browser? Create an account:"), "  "+ui.Link(m.accountURL("/register"), m.accountURL("/register")),
			ui.Dim.Render("Forgot your password?"), "  "+ui.Link(m.accountURL("/forgot-password"), m.accountURL("/forgot-password")))
	}
	return lines
}

// submitAccountForm checks the form that is showing, and sends it.
func (m *Model) submitAccountForm() {
	if m.authBusy {
		return
	}
	if !m.online {
		m.authError = "not connected to the server yet"
		return
	}
	email := strings.ToLower(strings.TrimSpace(m.emailInput.Value()))
	password, again := m.passwordInput.Value(), m.confirmInput.Value()
	server := m.server
	m.authError, m.authNotice = "", ""

	switch m.authMode {
	case modeRegister:
		name, handle := strings.TrimSpace(m.nameInput.Value()), strings.ToLower(strings.TrimSpace(m.handleInput.Value()))
		switch {
		case name == "":
			m.authError = "enter your name"
		case !crypto.ValidHandle(handle):
			m.authError = "the handle must be 3-20 characters: a-z, 0-9 or _"
		case !strings.Contains(email, "@") || !strings.Contains(email[strings.Index(email, "@"):], "."):
			m.authError = "that does not look like an email address"
		case len(password) < 8:
			m.authError = "the password must be at least 8 characters"
		case password != again:
			m.authError = "the two passwords are not the same"
		}
		if m.authError != "" {
			return
		}
		m.authBusy = true
		go func() {
			server.report(registerResult{email: email, password: password, err: server.Register(name, handle, email, password)})
		}()

	case modeForgot:
		if !strings.Contains(email, "@") {
			m.authError = "enter the email you registered with"
			return
		}
		m.authBusy = true
		go func() { server.report(forgotResult{email: email, err: server.ForgotPassword(email)}) }()

	case modeReset:
		code := strings.TrimSpace(m.codeInput.Value())
		switch {
		case !strings.Contains(email, "@"):
			m.authError = "enter the email the code was sent to"
		case code == "":
			m.authError = "enter the code from the email"
		case len(password) < 8:
			m.authError = "the password must be at least 8 characters"
		case password != again:
			m.authError = "the two passwords are not the same"
		}
		if m.authError != "" {
			return
		}
		m.authBusy = true
		go func() { server.report(resetResult{email: email, err: server.ResetPassword(email, code, password)}) }()
	}
}

// handleRegistered: the account exists, so log in with it at once.
func (m *Model) handleRegistered(result registerResult) {
	m.authBusy = false
	if result.err != nil {
		m.authError = result.err.Error()
		return
	}
	m.goToAuth(modeLogin)
	m.emailInput.SetValue(result.email)
	m.passwordInput.SetValue(result.password)
	m.authNotice = "Account created. Logging in..."
	m.submitAuth()
}

// handleForgot: a code is on its way (if the email has an account), so ask for it.
func (m *Model) handleForgot(result forgotResult) {
	m.authBusy = false
	if result.err != nil {
		m.authError = result.err.Error()
		return
	}
	m.goToAuth(modeReset)
	m.emailInput.SetValue(result.email)
	m.focusField(1)
	m.authNotice = "If that email has an account, a code is on its way. It can take a minute."
}

// handleReset: the password changed, so back to the login screen to use it.
func (m *Model) handleReset(result resetResult) {
	m.authBusy = false
	if result.err != nil {
		m.authError = result.err.Error()
		return
	}
	m.goToAuth(modeLogin)
	m.emailInput.SetValue(result.email)
	m.focusField(1)
	m.authNotice = "Password changed. Log in with your new password."
}
