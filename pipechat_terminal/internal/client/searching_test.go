package client

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"pipechat/internal/crypto"
)

func say(g *groupState, handle string, text string, when time.Time) {
	g.entries = append(g.entries, entry{when: when, handle: handle, text: text})
}

// twoChats is a logged-in screen with a group called "crew" and a private chat with bobby.
func twoChats(t *testing.T) (*Model, *groupState, *groupState) {
	m := loggedInModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 60})
	secret, _ := crypto.NewSecret("crew")
	crew := &groupState{secret: secret}
	direct := &groupState{secret: &crypto.Secret{Name: "@bobby"}, direct: true, peer: "bobby"}
	m.groups = append(m.groups, crew, direct)
	day := time.Date(2026, 10, 7, 9, 30, 0, 0, time.UTC)
	say(crew, "alice", "Deploy goes out on Friday", day)
	say(crew, "bob", "lunch at noon?", day.Add(time.Minute))
	say(crew, "alice", "friday works for the demo too", day.Add(2*time.Minute))
	say(direct, "bobby", "are you free FRIDAY evening", day.Add(3*time.Minute))
	say(direct, "ann", "no, sorry", day.Add(4*time.Minute))
	m.switchTo(1)
	return m, crew, direct
}

func searchFor(m *Model, argument string) string {
	m.input.SetValue("/search " + argument)
	m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.showHelp {
		return ""
	}
	view := m.View()
	m.updateChat(tea.KeyMsg{Type: tea.KeyEsc})
	return view
}

func TestSearchAcrossAllChats(t *testing.T) {
	m, _, _ := twoChats(t)
	view := searchFor(m, "friday")
	for _, want := range []string{"Search: friday", "3 matches in 2 chat(s)", "crew", "@bobby", "Deploy goes out on Friday", "friday works for the demo", "are you free FRIDAY evening", "@alice", "09:30"} {
		if !strings.Contains(view, want) {
			t.Errorf("results lack %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "lunch") || strings.Contains(view, "sorry") {
		t.Errorf("lines without the text must not be listed:\n%s", view)
	}
	if m.showHelp {
		t.Fatal("Esc closes the results")
	}
}

func TestSearchWithinOneChat(t *testing.T) {
	m, _, _ := twoChats(t)
	if view := searchFor(m, "-here friday"); !strings.Contains(view, "2 matches in 1 chat(s)") || strings.Contains(view, "FRIDAY evening") {
		t.Errorf("-here should stay in crew:\n%s", view)
	}
	if view := searchFor(m, "-in bobby friday"); !strings.Contains(view, "1 match in 1 chat(s)") || !strings.Contains(view, "FRIDAY evening") || strings.Contains(view, "Deploy") {
		t.Errorf("-in bobby should look only there:\n%s", view)
	}
	if view := searchFor(m, "-in cr demo"); !strings.Contains(view, "friday works for the demo") {
		t.Errorf("the start of a chat's name is enough:\n%s", view)
	}
	if view := searchFor(m, "-in crew nothing-said-this"); !strings.Contains(view, "0 matches") || !strings.Contains(view, "Nothing said") {
		t.Errorf("no match should say so:\n%s", view)
	}

	// Mistakes get a message on the chat, not a page.
	for input, want := range map[string]string{"": "usage: /search", "-in": "usage: /search", "-in nobody-here hello": "no chat called nobody-here"} {
		before := len(m.groups[m.current].entries)
		m.input.SetValue("/search " + input)
		m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
		if m.showHelp || !strings.Contains(entriesText(m.groups[m.current]), want) || len(m.groups[m.current].entries) == before {
			t.Errorf("/search %q should say %q", input, want)
		}
	}
	m.switchTo(0)
	m.input.SetValue("/search -here hello")
	m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
	if m.showHelp || !strings.Contains(entriesText(m.groups[0]), "welcome page") {
		t.Error("the welcome page has no messages to search")
	}
}

// A group that was closed can still be searched: its saved messages are opened with the key the account keeps.
func TestSearchFindsMessagesInClosedGroups(t *testing.T) {
	m := loggedInModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 60})
	m.store, _ = LoadStore(t.TempDir() + "/client.json")
	secret, _ := crypto.NewSecret("old-crew")
	for index, body := range []string{"the picnic is on Sunday", "bring chairs"} {
		plain, _ := json.Marshal(payload{Kind: "msg", Handle: "alice", Body: body})
		sealed, _ := secret.Seal(plain)
		m.store.History.Add(m.seqKey(secret.GIDHex()), uint64(index+1), time.Now().UnixMilli(), sealed)
	}
	m.vault.Closed = []closedChat{{Name: "old-crew", Code: secret.Code()}}

	view := searchFor(m, "picnic")
	for _, want := range []string{"1 match in 1 chat(s)", "old-crew (closed)", "the picnic is on Sunday"} {
		if !strings.Contains(view, want) {
			t.Errorf("results lack %q:\n%s", want, view)
		}
	}
	if view := searchFor(m, "-in old-crew chairs"); !strings.Contains(view, "bring chairs") {
		t.Errorf("a closed chat can be named:\n%s", view)
	}
}

func TestSearchListsAtMostSomeAndSaysHowManyMatched(t *testing.T) {
	m, crew, _ := twoChats(t)
	for i := 0; i < searchShown+50; i++ {
		say(crew, "alice", "needle in a haystack", time.Now())
	}
	m.input.SetValue("/search needle")
	m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
	if view := m.View(); !strings.Contains(view, "250 matches") {
		t.Errorf("the top of the page gives the full count:\n%s", view[:min(len(view), 300)])
	}
	full := m.pageContent(100) // the whole page, which is longer than the screen
	if !strings.Contains(full, "Showing the first 200 of 250") || strings.Count(full, "needle in a haystack") != searchShown {
		t.Errorf("the page lists %d lines and says it is cut:\n...%s", strings.Count(full, "needle in a haystack"), full[max(0, len(full)-200):])
	}
}
