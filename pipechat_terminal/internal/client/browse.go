package client

import (
	"fmt"
	"strings"

	"pipechat/internal/client/ui"
)

// browseRows is how many names the folder listing shows at once.
const browseRows = 8

// browsePanel lists the files and folders the path being typed after /upload, /download or /save could
// complete to, so the folder can be browsed instead of guessed: Up and Down move the highlight, Tab
// goes into the highlighted folder or takes the file. It is empty when no path is being typed.
func (m *Model) browsePanel() []string {
	if !takesPath(m.input.Value()) {
		return nil
	}
	matches := m.input.MatchedSuggestions()
	if len(matches) == 0 {
		return nil
	}
	current := m.input.CurrentSuggestion()
	selected := 0
	for index, match := range matches {
		if match == current {
			selected = index
			break
		}
	}
	first := max(0, min(selected-browseRows/2, len(matches)-browseRows))
	last := min(len(matches), first+browseRows)

	rows := []string{ui.Dim.Render(fmt.Sprintf("  %d here  ·  ↑/↓ choose  ·  Tab opens a folder or takes a file", len(matches)))}
	for index := first; index < last; index++ {
		name, folder := suggestionName(matches[index])
		if folder {
			name += "/"
		}
		if index == selected {
			rows = append(rows, ui.Active.Render("▸ "+name))
		} else {
			rows = append(rows, ui.Dim.Render("  "+name))
		}
	}
	if last < len(matches) {
		rows = append(rows, ui.Dim.Render(fmt.Sprintf("  … %d more", len(matches)-last)))
	}
	return rows
}

// suggestionName is the last part of a suggested line's path, and whether it is a folder.
func suggestionName(line string) (string, bool) {
	_, path, _ := strings.Cut(line, " ")
	path = strings.TrimRight(path, `"'`)
	if open := strings.LastIndexAny(path, `"'`); open >= 0 {
		path = path[open+1:] // inside quotes the name may hold spaces
	} else if space := strings.LastIndex(path, " "); space >= 0 {
		path = path[space+1:]
	}
	folder := strings.HasSuffix(path, "/") || strings.HasSuffix(path, `\`)
	path = strings.TrimRight(path, `/\`)
	if cut := strings.LastIndexAny(path, `/\`); cut >= 0 {
		path = path[cut+1:]
	}
	return path, folder
}

// withBrowsePanel draws the folder listing over the bottom of the chat, just above the input.
func (m *Model) withBrowsePanel(body string) string {
	rows := m.browsePanel()
	if len(rows) == 0 {
		return body
	}
	lines := strings.Split(body, "\n")
	if len(rows) >= len(lines) {
		return body
	}
	return strings.Join(append(lines[:len(lines)-len(rows)], rows...), "\n")
}
