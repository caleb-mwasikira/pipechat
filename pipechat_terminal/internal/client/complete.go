package client

import (
	"strings"

	"pipechat/internal/client/complete"
)

// Command suggestions. As someone types a command, the rest of the first matching one shows in grey
// after what they typed; Tab (or the right arrow) finishes it, and Up and Down try the other matches.
// What to suggest is worked out in the complete package; this is the part that sits in the input.

// refreshSuggestions sets what the input can suggest: the commands, and the closed chats by name.
func (m *Model) refreshSuggestions() {
	suggestions := append([]string(nil), complete.Commands...)
	for _, chat := range m.vault.Closed {
		suggestions = append(suggestions, "/reopen "+chat.Name)
	}
	suggestions = append(suggestions, complete.Paths(m.input.Value())...)
	m.input.SetSuggestions(suggestions)
}

// suggestionShown says whether the input is showing a suggestion that Tab would accept.
func (m *Model) suggestionShown() bool {
	value := m.input.Value()
	if !strings.HasPrefix(value, "/") {
		return false
	}
	suggestion := m.input.CurrentSuggestion()
	return len(suggestion) > len(value) && strings.EqualFold(suggestion[:len(value)], value)
}
