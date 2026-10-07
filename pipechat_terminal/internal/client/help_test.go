package client

import (
	"os"
	"path/filepath"
	"pipechat/internal/client/music"
	"pipechat/internal/client/ui"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"pipechat/internal/crypto"
)

func loggedInModel(t *testing.T) *Model {
	store, _ := LoadStore(filepath.Join(t.TempDir(), "client.json"))
	m := NewModel(store, "")
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m.session = &pendingAuth{handle: "ann"}
	m.loggedIn = true
	m.input.Focus()
	for _, line := range ui.WelcomeLines("ann", "aaaa bbbb") {
		m.groups[0].entries = append(m.groups[0].entries, entry{text: line, raw: true})
	}
	m.refresh()
	return m
}

func TestWelcomeIsShownOnTheWelcomePage(t *testing.T) {
	view := loggedInModel(t).View()
	for _, want := range []string{"Welcome to Pipechat, @ann", "/create NAME", "/help", "aaaa bbbb"} {
		if !strings.Contains(view, want) {
			t.Errorf("welcome page lacks %q:\n%s", want, view)
		}
	}
}

func TestHelpTakesOverTheScreenAndEscReturns(t *testing.T) {
	m := loggedInModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 120}) // tall enough to show the whole guide
	m.input.SetValue("/help")
	m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})

	view := m.View()
	for _, want := range []string{"GROUPS", "/create NAME", "/upload PATH", "KEYS", "Esc, Enter or q to go back"} {
		if !strings.Contains(view, want) {
			t.Errorf("help lacks %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Welcome to Pipechat") {
		t.Error("the chat should be replaced by the guide")
	}
	t.Log("\n" + view)

	m.updateChat(tea.KeyMsg{Type: tea.KeyEsc})
	if m.showHelp || !strings.Contains(m.View(), "Welcome to Pipechat") {
		t.Error("Esc should return to the chat")
	}
}

func TestHelpFitsANarrowTerminal(t *testing.T) {
	m := loggedInModel(t)
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	m.input.SetValue("/help")
	m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
	for _, line := range strings.Split(m.View(), "\n") {
		if w := len([]rune(line)); w > 60 {
			t.Errorf("line is %d columns wide: %q", w, line)
		}
	}
}

func TestNoPlayerGivesAHelpfulMessage(t *testing.T) {
	previous := music.LookPath
	music.LookPath = func(string) (string, error) { return "", os.ErrNotExist }
	defer func() { music.LookPath = previous }()

	m := loggedInModel(t)
	m.input.SetValue("/play 1")
	m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
	if m.loadingMusic || !strings.Contains(entriesText(m.groups[0]), "install mpv") {
		t.Fatalf("expected advice to install a player:\n%s", entriesText(m.groups[0]))
	}
}

func TestCommandSuggestionsAppearAndTabFinishesThem(t *testing.T) {
	m := loggedInModel(t)
	typed := func(text string) {
		m.input.Reset()
		m.updateChat(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)}) // typed, so the input works out its matches
	}

	typed("/pl")
	if m.input.CurrentSuggestion() != "/play" || !m.suggestionShown() {
		t.Fatalf("suggestion for /pl: %q", m.input.CurrentSuggestion())
	}
	m.updateChat(tea.KeyMsg{Type: tea.KeyTab})
	if m.input.Value() != "/play" {
		t.Fatalf("Tab should finish the command, got %q", m.input.Value())
	}

	typed("/")
	if m.input.CurrentSuggestion() != "/help" {
		t.Fatalf("a bare / should suggest /help, got %q", m.input.CurrentSuggestion())
	}

	typed("/rep")
	m.updateChat(tea.KeyMsg{Type: tea.KeyDown}) // next match
	if got := m.input.CurrentSuggestion(); got != "/repeat one" && got != "/repeat off" {
		t.Fatalf("arrow keys should move through the matches, got %q", got)
	}

	// Plain text gets no suggestion, so Tab still switches chats.
	typed("hello")
	if m.suggestionShown() {
		t.Fatal("ordinary messages must not be completed")
	}
	m.groups = append(m.groups, &groupState{secret: nil})
	before := m.current
	m.updateChat(tea.KeyMsg{Type: tea.KeyTab})
	if m.current == before {
		t.Fatal("Tab should switch chats when there is nothing to complete")
	}
	if m.input.Value() != "hello" {
		t.Fatalf("Tab changed the message: %q", m.input.Value())
	}
}

func TestSidebarShowsWhoIsOnline(t *testing.T) {
	m := loggedInModel(t)
	secret, _ := crypto.NewSecret("crew")
	m.groups = append(m.groups, &groupState{secret: secret})
	m.switchTo(1)
	m.connected = true
	m.peersOnline = map[string][]string{secret.GIDHex(): {"bobby", "carol"}}

	view := m.View()
	for _, want := range []string{"Online (3)", "@ann (you)", "@bobby", "@carol"} {
		if !strings.Contains(view, want) {
			t.Errorf("the sidebar lacks %q:\n%s", want, view)
		}
	}
	m.switchTo(0)
	if strings.Contains(m.View(), "Online (") {
		t.Error("the welcome page has no list of who is online")
	}
}

func TestLoginScreenShowsWhereToRegister(t *testing.T) {
	store, _ := LoadStore(t.TempDir() + "/client.json")
	m := NewModel(store, "")
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.server = NewServer("https://chat.example.com")
	m.serverReached, m.online = true, true
	t.Cleanup(m.server.Stop)

	// Until the server says otherwise, the website is taken to be at the server's own address.
	view := m.authView()
	for _, want := range []string{"No account yet?", "https://chat.example.com/register", "Forgot your password?", "https://chat.example.com/forgot-password"} {
		if !strings.Contains(view, want) {
			t.Errorf("the login screen lacks %q:\n%s", want, view)
		}
	}

	// A server that names its website is believed.
	m.Update(infoResult{website: "https://www.example.org/"})
	view = m.authView()
	if !strings.Contains(view, "https://www.example.org/register") || strings.Contains(view, "chat.example.com/register") {
		t.Errorf("the named website should be used:\n%s", view)
	}
	// An answer with an error changes nothing.
	m.Update(infoResult{website: "https://elsewhere.test", err: os.ErrNotExist})
	if strings.Contains(m.authView(), "elsewhere.test") {
		t.Error("a failed lookup must not replace the address")
	}
	// The link is clickable in terminals that can (an OSC 8 link around the text).
	if !strings.Contains(m.authView(), "\x1b]8;;https://www.example.org/register") {
		t.Error("the address should be a hyperlink")
	}
}
