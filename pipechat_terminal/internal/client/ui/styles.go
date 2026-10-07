// Package ui is how the terminal client looks: the colours and styles, and the pure screens that are
// drawn from plain data (the help guide, the welcome page, a person's profile, the upload preview, the
// progress bar, the list of who is online). It knows nothing about the chat server or the model.
package ui

import "github.com/charmbracelet/lipgloss"

// SidebarWidth is the width of the list of chats on the left.
const SidebarWidth = 24

var (
	Title     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("62")).Padding(0, 1)
	Dim       = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	Error     = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	Self      = lipgloss.NewStyle().Foreground(lipgloss.Color("114")).Bold(true)
	Other     = lipgloss.NewStyle().Foreground(lipgloss.Color("111")).Bold(true)
	Active    = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	Sidebar   = lipgloss.NewStyle().Width(SidebarWidth).BorderRight(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240"))
	AuthBox   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")).Padding(1, 3)
	Brand     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("49"))
	Heading   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("49"))
	Command   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("87"))
	Key       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("222"))
	Text      = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	OnlineDot = lipgloss.NewStyle().Foreground(lipgloss.Color("49")).Render("●")
)

// Truncate shortens text to width characters, ending it with … when it was cut.
func Truncate(text string, width int) string {
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}
	return string(runes[:max(1, width-1)]) + "…"
}
