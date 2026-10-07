package client

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"pipechat/internal/client/music"
)

func typeKeys(m *Model, text string) {
	for _, character := range text {
		switch character {
		case '\n':
			m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
		case ' ':
			m.updateChat(tea.KeyMsg{Type: tea.KeySpace})
		default:
			m.updateChat(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{character}})
		}
	}
}

// aLibraryPage opens the music page with 60 songs, so there is a lot to search and scroll through.
func aLibraryPage(t *testing.T) *Model {
	m := loggedInModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	var tracks []music.Track
	for i := 1; i <= 60; i++ {
		tracks = append(tracks, music.Track{ID: int64(i), Title: fmt.Sprintf("Song %02d", i), Artist: "Band", Ext: "mp3", Size: 1 << 20, Uploader: "alice", MD5: fmt.Sprintf("%08x00000000000000000000000000", i*7919)})
	}
	tracks[44].Title = "Blue Hour"
	tracks[9].Title = "blue moon"
	m.Update(musicList{tracks: tracks})
	if !m.showHelp {
		t.Fatal("the music page should be open")
	}
	return m
}

func TestSearchingAPageLikeVim(t *testing.T) {
	m := aLibraryPage(t)
	if strings.Contains(m.View(), "Blue Hour") {
		t.Fatal("the song being searched for starts out of view")
	}

	typeKeys(m, "/")
	if !m.pageFind.typing || !strings.Contains(m.View(), "/█") {
		t.Fatalf("typing a search shows the prompt:\n%s", m.View())
	}
	typeKeys(m, "blue")
	if !strings.Contains(m.View(), "/blue█") {
		t.Fatal("what is typed shows on the bottom line")
	}
	typeKeys(m, "\n")
	view := m.View()
	// "blue" (no capitals) matches both, ignoring capitals, and the page jumped to the first at or after the top: blue moon.
	if m.pageFind.typing || len(m.pageFind.matches) != 2 || !strings.Contains(view, "match 1 of 2") || !strings.Contains(view, "blue moon") {
		t.Fatalf("search confirmed: %+v\n%s", m.pageFind, view)
	}

	// n goes to the next, which is further down and brought into view; N goes back; both wrap round.
	typeKeys(m, "n")
	if m.pageFind.at != 1 || !strings.Contains(m.View(), "match 2 of 2") || !strings.Contains(m.View(), "Blue Hour") {
		t.Fatalf("n should go to the second match:\n%s", m.View())
	}
	typeKeys(m, "n")
	if m.pageFind.at != 0 || !strings.Contains(m.View(), "blue moon") {
		t.Fatal("n wraps round to the first")
	}
	typeKeys(m, "N")
	if m.pageFind.at != 1 {
		t.Fatal("N goes to the one before, wrapping round")
	}

	// Esc clears the search, the next Esc leaves the page.
	typeKeys(m, "")
	m.updateChat(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.showHelp || m.pageFind.pattern != "" || strings.Contains(m.View(), "match ") {
		t.Fatal("the first Esc only clears the search")
	}
	m.updateChat(tea.KeyMsg{Type: tea.KeyEsc})
	if m.showHelp {
		t.Fatal("the second Esc closes the page")
	}
}

func TestPageSearchSmartCaseNotFoundAndCancel(t *testing.T) {
	m := aLibraryPage(t)

	// A capital makes it exact: "Blue" finds only "Blue Hour".
	typeKeys(m, "/Blue\n")
	if len(m.pageFind.matches) != 1 || !strings.Contains(m.View(), "match 1 of 1") || !strings.Contains(m.View(), "Blue Hour") {
		t.Fatalf("exact match:\n%s", m.View())
	}

	// Something that is not there says so, and n does nothing.
	typeKeys(m, "/no-such-song\n")
	if !strings.Contains(m.View(), "Pattern not found: no-such-song") {
		t.Fatalf("not found:\n%s", m.View())
	}
	before := m.helpView.YOffset
	typeKeys(m, "n")
	if m.helpView.YOffset != before {
		t.Fatal("n with no match must not move")
	}

	// Esc while typing gives up the search without touching the last one; Backspace edits, and on an empty line leaves it.
	typeKeys(m, "/abc")
	m.updateChat(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.pageFind.buffer != "ab" {
		t.Fatalf("backspace: %q", m.pageFind.buffer)
	}
	m.updateChat(tea.KeyMsg{Type: tea.KeyEsc})
	if m.pageFind.typing || !m.showHelp || m.pageFind.pattern != "no-such-song" {
		t.Fatalf("Esc while typing only cancels the typing: %+v", m.pageFind)
	}
	typeKeys(m, "/")
	m.updateChat(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.pageFind.typing {
		t.Fatal("Backspace on an empty search line leaves it")
	}
	// An empty search does nothing; keys typed into the search do not leave the page or move it.
	typeKeys(m, "/\n")
	typeKeys(m, "/qn \n") // "q" and "n" and a space are text here, not commands
	if !m.showHelp {
		t.Fatal("letters typed into a search must not close the page")
	}
}

func TestPageSearchWorksOnTheFilesPageAndAHashCanBeFound(t *testing.T) {
	m := aLibraryPage(t)
	hash := fmt.Sprintf("%08x", 33*7919)
	typeKeys(m, "/"+hash+"\n")
	if len(m.pageFind.matches) != 1 || !strings.Contains(m.View(), "Song 33") {
		t.Fatalf("a hash can be searched for:\n%s", m.View())
	}

	// The same keys work in the help guide and the files page.
	m.updateChat(tea.KeyMsg{Type: tea.KeyEsc})
	m.updateChat(tea.KeyMsg{Type: tea.KeyEsc})
	m.input.SetValue("/help")
	m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
	typeKeys(m, "/shuffle\n")
	if len(m.pageFind.matches) == 0 || !strings.Contains(m.View(), "match 1 of") {
		t.Fatalf("searching the help guide:\n%s", m.View())
	}
	typeKeys(m, "g")
	if m.helpView.YOffset != 0 {
		t.Fatal("g goes to the top")
	}
	typeKeys(m, "G")
	if !m.helpView.AtBottom() {
		t.Fatal("G goes to the bottom")
	}
}

func TestResizingKeepsTheSearchMarks(t *testing.T) {
	m := aLibraryPage(t)
	typeKeys(m, "/Blue\n")
	m.Update(tea.WindowSizeMsg{Width: 70, Height: 25})
	if m.pageFind.pattern != "Blue" || len(m.pageFind.matches) != 1 || !strings.Contains(m.View(), "match 1 of 1") {
		t.Fatalf("a resize redraws the page and keeps the search: %+v", m.pageFind)
	}
}
