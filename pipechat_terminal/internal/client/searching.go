package client

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"pipechat/internal/client/search"
	"pipechat/internal/client/ui"
	"pipechat/internal/crypto"
)

// Searching what was said. /search looks through the chats that are open and the groups that were closed,
// using what is on this computer: nothing is asked of the server, so nothing about the search leaves it.

const searchShown = 200 // matches listed; the count of all of them is still given

// chatName is how a chat is called in the results.
func chatName(g *groupState) string {
	if g.direct {
		return "@" + g.peer
	}
	return g.name()
}

// chatLines are the things said in an open chat, as the screen shows them.
func chatLines(g *groupState) []search.Line {
	lines := make([]search.Line, 0, len(g.entries))
	for _, entry := range g.entries {
		lines = append(lines, search.Line{Chat: chatName(g), When: entry.when, Handle: entry.handle, Text: entry.text})
	}
	return lines
}

// closedLines reads a closed group's saved messages. They are stored sealed, so they are opened with the
// group's key, which this account still has.
func (m *Model) closedLines(chat closedChat) []search.Line {
	if chat.Code == "" {
		return nil
	}
	secret, err := crypto.ParseCode(chat.Code)
	if err != nil {
		return nil
	}
	var lines []search.Line
	for _, saved := range m.store.History.Messages(m.seqKey(secret.GIDHex())) {
		plain, err := secret.Open(saved.CT)
		var content payload
		if err != nil || json.Unmarshal(plain, &content) != nil {
			continue
		}
		handle, text := cleanText(content.Handle), cleanText(content.Body)
		switch content.Kind {
		case "file":
			handle, text = "", fmt.Sprintf("@%s shared \"%s\"", cleanText(content.Handle), content.Name)
		case "join":
			handle, text = "", "@"+cleanText(content.Handle)+" joined the group"
		}
		lines = append(lines, search.Line{Chat: chat.Name + " (closed)", When: time.UnixMilli(saved.TS), Handle: handle, Text: text})
	}
	return lines
}

// searchCommand handles /search.
func (m *Model) searchCommand(current *groupState, argument string) tea.Cmd {
	request, err := search.Parse(argument)
	if err != nil {
		m.notice(current, err.Error())
		return nil
	}

	// Which chats to look in.
	var chats [][]search.Line
	switch request.Scope {
	case search.CurrentChat:
		if current.secret == nil {
			m.notice(current, "this is the welcome page; open a chat first, or search every chat with /search TEXT")
			return nil
		}
		chats = append(chats, chatLines(current))
	case search.NamedChat:
		for _, g := range m.groups[1:] {
			if search.ChatMatches(chatName(g), request.Chat) {
				chats = append(chats, chatLines(g))
			}
		}
		for _, closed := range m.vault.Closed {
			if search.ChatMatches(closed.Name, request.Chat) {
				chats = append(chats, m.closedLines(closed))
			}
		}
		if len(chats) == 0 {
			m.notice(current, "no chat called "+request.Chat+"; /groups lists them, and /closed the ones you closed")
			return nil
		}
	default:
		for _, g := range m.groups[1:] {
			chats = append(chats, chatLines(g))
		}
		for _, closed := range m.vault.Closed {
			chats = append(chats, m.closedLines(closed))
		}
	}

	var groups []ui.SearchGroup
	total, shown := 0, 0
	for _, lines := range chats {
		hits, count := search.Find(lines, request.Query, searchShown-shown)
		total += count
		shown += len(hits)
		if count > 0 {
			groups = append(groups, ui.SearchGroup{Chat: hits[0].Chat, Hits: hits})
		}
	}
	// A chat that matched but had no room left in the list still counts, and the page says so.
	query := request.Query
	command := m.openPage("search "+query, func(width int) string {
		return ui.SearchResults(width, strings.TrimSpace(query), groups, total, shown)
	})
	m.pageFooter = "↑/↓ scroll  ·  / search again  ·  Esc back  ·  select text to copy" // after openPage, which clears it
	return command
}
