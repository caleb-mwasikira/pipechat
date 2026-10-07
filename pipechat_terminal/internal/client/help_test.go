package client

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func loggedInModel(t *testing.T) *Model {
	store, _ := LoadStore(filepath.Join(t.TempDir(), "client.json"))
	m := NewModel(store, "")
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m.session = &pendingAuth{handle: "ann"}
	m.loggedIn = true
	for _, line := range welcomeLines("ann", "aaaa bbbb") {
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
