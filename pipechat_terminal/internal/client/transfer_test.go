package client

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"pipechat/internal/client/files"
)

func TestTheBarTakesALineFromTheChatWhileTransferring(t *testing.T) {
	m := loggedInModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	before := m.viewport.Height
	m.busy = true
	m.Update(files.Progress{Verb: "uploading", Name: "big.iso", Done: 1 << 20, Total: 4 << 20, Rate: 1 << 20})
	if m.viewport.Height != before-1 {
		t.Fatalf("the chat should give up a line: %d -> %d", before, m.viewport.Height)
	}
	view := m.View()
	if !strings.Contains(view, "big.iso") || !strings.Contains(view, " 25%") {
		t.Fatalf("the view should show the bar:\n%s", view)
	}
	if lines := strings.Count(view, "\n") + 1; lines != 30 {
		t.Fatalf("the screen should stay 30 lines, it is %d", lines)
	}

	// A tick that arrives after the transfer ended must not bring the bar back.
	m.finishUpload(files.Done{Name: "big.iso", Err: errBoom})
	m.Update(files.Progress{Verb: "uploading", Name: "big.iso", Done: 2 << 20, Total: 4 << 20})
	if m.progress != nil || m.viewport.Height != before || strings.Contains(m.View(), "25%") {
		t.Fatal("the bar should be gone once the transfer is over")
	}
}

type boom struct{}

func (boom) Error() string { return "boom" }

var errBoom error = boom{}
