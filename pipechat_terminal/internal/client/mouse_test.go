package client

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"pipechat/internal/client/ui"
)

func TestPagesLetGoOfTheMouseSoTextCanBeCopied(t *testing.T) {
	m := loggedInModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if !m.mouseCaptured || !m.mouseWanted {
		t.Fatal("the chat starts with the mouse wheel on")
	}

	// The music and files pages free the mouse, so the terminal can select hashes and names...
	m.Update(musicList{tracks: nil})
	if !m.showHelp || m.mouseCaptured {
		t.Fatalf("the music page should release the mouse: page %v, captured %v", m.showHelp, m.mouseCaptured)
	}
	if !strings.Contains(m.View(), "select text to copy") {
		t.Fatalf("the page says text can be selected:\n%s", m.View())
	}
	// ...and the chat takes it back when the page closes.
	m.updateChat(tea.KeyMsg{Type: tea.KeyEsc})
	if m.showHelp || !m.mouseCaptured {
		t.Fatalf("closing the page returns the wheel: page %v, captured %v", m.showHelp, m.mouseCaptured)
	}

	m.input.SetValue("/files")
	m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.showHelp || m.mouseCaptured || !strings.Contains(m.View(), "select text to copy") {
		t.Fatal("the files page also lets text be selected")
	}
	m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.mouseCaptured {
		t.Fatal("any way of closing a page restores the wheel")
	}
}

func TestMouseCommandAndTheChoiceIsRemembered(t *testing.T) {
	m := loggedInModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	m.input.SetValue("/mouse off")
	m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mouseCaptured || m.mouseWanted || !m.store.MouseOff || !strings.Contains(entriesText(m.groups[0]), "select and copy text freely") {
		t.Fatalf("mouse off: captured %v wanted %v remembered %v", m.mouseCaptured, m.mouseWanted, m.store.MouseOff)
	}
	// With it off, opening and closing a page must not turn it on again.
	m.Update(musicList{})
	m.updateChat(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mouseCaptured {
		t.Fatal("a closed page must not undo /mouse off")
	}

	// It comes back with /mouse on, and with a bare /mouse it flips.
	m.input.SetValue("/mouse on")
	m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.mouseCaptured || m.store.MouseOff || !strings.Contains(entriesText(m.groups[0]), "hold Shift") {
		t.Fatal("mouse on")
	}
	m.input.SetValue("/mouse")
	m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mouseCaptured {
		t.Fatal("a bare /mouse flips it")
	}
	m.input.SetValue("/mouse maybe")
	m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(entriesText(m.groups[0]), "usage: /mouse on|off") {
		t.Fatal("a mistake gets the usage")
	}

	// A new run starts the way the person left it.
	again := NewModel(m.store, "")
	if again.mouseWanted || again.mouseCaptured {
		t.Fatal("the next run should start with the mouse off")
	}
	if guide := ui.HelpGuide(100); !strings.Contains(guide, "/mouse [on|off]") {
		t.Fatal("the help guide lists /mouse")
	}
}
