package client

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"pipechat/internal/client/music"
	"pipechat/internal/client/ui"
	"pipechat/internal/crypto"
)

// aLongChat is a screen 100 columns by 20 lines with 100 messages in the open chat.
func aLongChat(t *testing.T) (*Model, *groupState) {
	m := loggedInModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	secret, _ := crypto.NewSecret("crew")
	g := &groupState{secret: secret}
	m.groups = append(m.groups, g)
	for i := 1; i <= 100; i++ {
		say(g, "alice", fmt.Sprintf("message number %03d", i), time.Date(2026, 10, 7, 9, 0, i, 0, time.UTC))
	}
	m.switchTo(1)
	return m, g
}

func wheel(m *Model, button tea.MouseButton) {
	m.Update(tea.MouseMsg(tea.MouseEvent{Button: button, Action: tea.MouseActionPress}))
}

func TestAChatOpensAtItsNewestMessageAndCanBeScrolled(t *testing.T) {
	m, _ := aLongChat(t)
	if view := m.View(); !strings.Contains(view, "message number 100") || strings.Contains(view, "message number 001") || strings.Contains(view, "reading older") {
		t.Fatalf("a chat opens at the bottom:\n%s", view)
	}

	// The wheel, the arrows and the page keys each go up.
	wheel(m, tea.MouseButtonWheelUp)
	afterWheel := m.viewport.YOffset
	if m.viewport.AtBottom() {
		t.Fatal("the wheel should scroll up")
	}
	m.updateChat(tea.KeyMsg{Type: tea.KeyUp})
	if m.viewport.YOffset != afterWheel-1 {
		t.Fatalf("up arrow moves one line: %d -> %d", afterWheel, m.viewport.YOffset)
	}
	m.updateChat(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.viewport.YOffset >= afterWheel-1 {
		t.Fatal("page up moves a page")
	}
	view := m.View()
	if !strings.Contains(view, "reading older") || !strings.Contains(view, "Esc: latest") || strings.Contains(view, "message number 100") {
		t.Fatalf("scrolled up, the title says so and the newest is out of view:\n%s", view)
	}

	// And back down, by the wheel, the arrows, or at once with Esc.
	m.updateChat(tea.KeyMsg{Type: tea.KeyDown})
	wheel(m, tea.MouseButtonWheelDown)
	m.updateChat(tea.KeyMsg{Type: tea.KeyPgDown})
	m.updateChat(tea.KeyMsg{Type: tea.KeyPgUp})
	m.updateChat(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.viewport.AtBottom() || strings.Contains(m.View(), "reading older") || !strings.Contains(m.View(), "message number 100") {
		t.Fatal("Esc returns to the newest message")
	}
	// Scrolling can go all the way to the first message.
	for range 60 {
		m.updateChat(tea.KeyMsg{Type: tea.KeyPgUp})
	}
	if !strings.Contains(m.View(), "message number 001") {
		t.Fatalf("the top of the chat can be reached:\n%s", m.View())
	}
}

func TestNewMessagesDoNotPullYouAwayFromWhatYouAreReading(t *testing.T) {
	m, g := aLongChat(t)
	m.updateChat(tea.KeyMsg{Type: tea.KeyPgUp})
	reading := m.viewport.YOffset

	for i := 0; i < 3; i++ {
		plain := []byte(fmt.Sprintf(`{"k":"msg","h":"bob","b":"fresh %d"}`, i))
		sealed, _ := g.secret.Seal(plain)
		m.handleEvent(Event{Type: "msg", GID: g.secret.GIDHex(), CT: sealed, Seq: uint64(1000 + i), TS: time.Now().UnixMilli()})
	}
	if m.viewport.YOffset != reading {
		t.Fatalf("the page moved from %d to %d while reading", reading, m.viewport.YOffset)
	}
	if m.newBelow != 3 || !strings.Contains(m.View(), "3 new below") {
		t.Fatalf("it should say 3 new messages wait below (%d):\n%s", m.newBelow, m.View())
	}
	m.updateChat(tea.KeyMsg{Type: tea.KeyEsc})
	if m.newBelow != 0 || !strings.Contains(m.View(), "fresh 2") {
		t.Fatal("Esc shows the new messages and clears the note")
	}

	// A message while at the bottom just appears, and a chat opened again starts at its bottom.
	m.updateChat(tea.KeyMsg{Type: tea.KeyPgUp})
	m.switchTo(0)
	m.switchTo(1)
	if !m.viewport.AtBottom() || m.newBelow != 0 {
		t.Fatal("switching back to a chat opens it at the newest message")
	}
}

func TestArrowsStillChooseSuggestionsWhenThereAreSome(t *testing.T) {
	m, _ := aLongChat(t)
	for _, character := range "/rep" {
		m.updateChat(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{character}})
	}
	if !m.suggestionShown() {
		t.Fatal("a suggestion should be showing")
	}
	before := m.viewport.YOffset
	m.updateChat(tea.KeyMsg{Type: tea.KeyDown})
	if m.viewport.YOffset != before {
		t.Fatal("with suggestions showing, the arrows pick one instead of scrolling")
	}
	m.input.Reset()
	m.updateChat(tea.KeyMsg{Type: tea.KeyUp})
	if m.viewport.YOffset >= before {
		t.Fatal("with nothing to suggest, up scrolls")
	}
}

func TestThePageScrollsWithTheWheelToo(t *testing.T) {
	m := loggedInModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 12})
	m.input.SetValue("/help")
	m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.showHelp {
		t.Fatal("help should be open")
	}
	wheel(m, tea.MouseButtonWheelDown)
	if m.helpView.YOffset == 0 {
		t.Fatal("the wheel scrolls the help page")
	}
}

func TestTheMusicPageMarksWhatIsPlayingAndFitsTheWidth(t *testing.T) {
	tracks := []music.Track{
		{ID: 1, Title: "First", Artist: "Band", Ext: "mp3", Size: 4 << 20, Uploader: "alice", MD5: "9e107d9d372bb6826bd81d3542a419d6"},
		{ID: 22, Title: strings.Repeat("A very long title ", 8), Ext: "flac", Size: 31 << 20, Uploader: "a-very-long-handle-indeed"},
	}
	page := ui.MusicLibrary(80, tracks, 22, " [shuffle]")
	plain := stripOSC(page)
	for _, want := range []string{"2 songs [shuffle]", "Band - First.mp3", "9e107d9d", "4.0 MB", "@alice", "♪ ", "/play SONG", "/save all"} {
		if !strings.Contains(plain, want) {
			t.Errorf("page lacks %q:\n%s", want, plain)
		}
	}
	for _, line := range strings.Split(plain, "\n") {
		if w := len([]rune(line)); w > 80 {
			t.Errorf("a line is %d columns wide: %q", w, line)
		}
	}
	empty := stripOSC(ui.MusicLibrary(80, nil, 0, ""))
	if !strings.Contains(empty, "The library is empty") || !strings.Contains(empty, "website") {
		t.Errorf("empty library:\n%s", empty)
	}
}

// stripOSC removes the colour codes, so widths can be counted.
func stripOSC(text string) string {
	var out strings.Builder
	skipping := false
	for _, character := range text {
		switch {
		case character == 0x1b:
			skipping = true
		case skipping && character == 'm':
			skipping = false
		case !skipping:
			out.WriteRune(character)
		}
	}
	return out.String()
}

func TestTheFilesPageFitsTheWidthAndSaysWhenThereAreNone(t *testing.T) {
	at := time.Date(2026, 10, 7, 14, 5, 0, 0, time.UTC)
	page := stripOSC(ui.SharedFiles(80, []ui.SharedFile{
		{Number: 1, Name: "holiday-photos.zip", Hash: "9e107d9d372bb6826bd81d3542a419d6", Size: 12 << 20, From: "bobby", At: at},
		{Number: 2, Name: strings.Repeat("long-name-", 10) + ".tar.gz", Size: 3 << 10, At: at},
	}))
	for _, want := range []string{"2 files", "holiday-photos.zip", "9e107d9d", "12.0 MB", "@bobby", "you", "07 Oct 14:05", "/download FILE", "/share FILE"} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q:\n%s", want, page)
		}
	}
	for _, line := range strings.Split(page, "\n") {
		if w := len([]rune(line)); w > 80 {
			t.Errorf("a line is %d columns wide: %q", w, line)
		}
	}
	if empty := stripOSC(ui.SharedFiles(80, nil)); !strings.Contains(empty, "Nothing has been shared yet") || !strings.Contains(empty, "/upload PATH") {
		t.Errorf("empty list:\n%s", empty)
	}
}
