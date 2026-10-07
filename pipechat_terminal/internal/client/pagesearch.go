package client

import (
	"fmt"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"pipechat/internal/client/search"
	"pipechat/internal/client/ui"
)

// Searching inside a full-screen page (help, music, files, search results), the way vim does:
//
//	/text    start a search: type the text, Enter to go to the first match, Esc to give up
//	n  N     the next match, the one before (they wrap round)
//	Esc      clear the marks (a second Esc leaves the page)
//	g  G     the top, the bottom
//
// As in vim's smartcase, capitals in the text make the search exact; without them, capitals do not matter.

type pageSearch struct {
	typing  bool   // the text is being typed on the bottom line
	buffer  string // what has been typed so far
	pattern string // the confirmed search; empty when none
	matches []int  // the lines that hold it
	at      int    // which of matches is the one looked at
}

var (
	pageMark   = lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("222"))
	pageStrong = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("16")).Background(lipgloss.Color("49"))
)

// renderPage draws the page into its viewport, with the search marks if there is a search.
func (m *Model) renderPage() {
	m.pageBase = m.pageContent(m.helpView.Width)
	content := m.pageBase
	if m.pageFind.pattern != "" {
		m.pageFind.matches = search.PageMatches(m.pageBase, m.pageFind.pattern)
		current := -1
		if m.pageFind.at >= 0 && m.pageFind.at < len(m.pageFind.matches) {
			current = m.pageFind.matches[m.pageFind.at]
		}
		content = search.PageHighlight(m.pageBase, m.pageFind.pattern, current,
			func(text string) string { return pageMark.Render(text) }, func(text string) string { return pageStrong.Render(text) })
	}
	m.helpView.SetContent(content)
}

// pageKey handles the keys of a page that has none of its own; it says whether it used the key.
func (m *Model) pageKey(key tea.KeyMsg) (bool, tea.Cmd) {
	if m.pageFind.typing {
		m.typeSearch(key)
		return true, nil
	}
	switch key.String() {
	case "/":
		m.pageFind.typing, m.pageFind.buffer = true, ""
		return true, nil
	case "n", "N":
		if m.pageFind.pattern == "" {
			return true, nil
		}
		direction := 1
		if key.String() == "N" {
			direction = -1
		}
		m.pageFind.at = search.Step(m.pageFind.matches, m.pageFind.at, direction)
		m.lookAtMatch()
		return true, nil
	case "g":
		m.helpView.GotoTop()
		return true, nil
	case "G":
		m.helpView.GotoBottom()
		return true, nil
	case "esc":
		if m.pageFind.pattern != "" { // first Esc clears the marks, as :noh does
			m.pageFind = pageSearch{}
			m.renderPage()
			return true, nil
		}
		m.showHelp = false
		return true, m.pageClosed()
	case "q", "enter", "ctrl+h":
		m.showHelp = false
		return true, m.pageClosed()
	}
	return false, nil
}

// typeSearch handles a key while the text of a search is being typed.
func (m *Model) typeSearch(key tea.KeyMsg) {
	switch key.Type {
	case tea.KeyEnter:
		m.pageFind.typing = false
		if m.pageFind.buffer == "" {
			return
		}
		m.pageFind.pattern = m.pageFind.buffer
		m.pageFind.matches = search.PageMatches(m.pageBase, m.pageFind.pattern)
		m.pageFind.at = search.FirstFrom(m.pageFind.matches, m.helpView.YOffset)
		m.renderPage()
		m.lookAtMatch()
	case tea.KeyEsc, tea.KeyCtrlC:
		m.pageFind.typing, m.pageFind.buffer = false, ""
	case tea.KeyBackspace:
		if m.pageFind.buffer == "" {
			m.pageFind.typing = false // Backspace on an empty line leaves it, as in vim
			return
		}
		_, size := utf8.DecodeLastRuneInString(m.pageFind.buffer)
		m.pageFind.buffer = m.pageFind.buffer[:len(m.pageFind.buffer)-size]
	case tea.KeyCtrlU:
		m.pageFind.buffer = ""
	case tea.KeyRunes, tea.KeySpace:
		m.pageFind.buffer += string(key.Runes)
		if key.Type == tea.KeySpace {
			m.pageFind.buffer += " "
		}
	}
}

// lookAtMatch scrolls so the match being looked at is in view, a little below the top.
func (m *Model) lookAtMatch() {
	m.renderPage() // the match looked at is marked more strongly
	if m.pageFind.at < 0 || m.pageFind.at >= len(m.pageFind.matches) {
		return
	}
	line := m.pageFind.matches[m.pageFind.at]
	if line < m.helpView.YOffset || line >= m.helpView.YOffset+m.helpView.Height {
		m.helpView.SetYOffset(max(0, line-m.helpView.Height/3))
	}
}

// pageFooterLine is the bottom line of a page: the search being typed, the state of a search, or the usual hints.
func (m *Model) pageFooterLine() string {
	switch {
	case m.pageFind.typing:
		return ui.Active.Render("/") + ui.Text.Render(m.pageFind.buffer) + ui.Brand.Render("█") + ui.Dim.Render("   Enter: find · Esc: cancel")
	case m.pageFind.pattern != "" && len(m.pageFind.matches) == 0:
		return ui.Error.Render("Pattern not found: "+m.pageFind.pattern) + ui.Dim.Render("   · / search again · Esc clear")
	case m.pageFind.pattern != "":
		return ui.Key.Render(fmt.Sprintf("/%s  match %d of %d", m.pageFind.pattern, m.pageFind.at+1, len(m.pageFind.matches))) +
			ui.Dim.Render("   · n next · N previous · Esc clear")
	case m.pageFooter != "":
		return ui.Key.Render(m.pageFooter)
	}
	return ui.Dim.Render("↑/↓ scroll  ·  / search  ·  Esc, Enter or q to go back")
}
