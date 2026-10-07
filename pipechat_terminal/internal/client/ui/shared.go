package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"pipechat/internal/client/files"
	"pipechat/internal/client/lookup"
)

// SharedFile is one file somebody shared in a chat.
type SharedFile struct {
	Number int64
	Name   string
	Hash   string
	Size   int64
	From   string // a handle, or empty for what you shared yourself
	At     time.Time
}

// SharedFiles draws the files shared in chats the way the help guide is drawn: a heading, aligned columns,
// and the commands that go with it.
func SharedFiles(width int, list []SharedFile) string {
	const (
		numberWidth = 5
		hashWidth   = 8
		sizeWidth   = 9
		fromWidth   = 12
		whenWidth   = 12
		gaps        = 5 * 2
	)
	fileWidth := max(14, width-4-numberWidth-hashWidth-sizeWidth-fromWidth-whenWidth-gaps)
	grey := lipgloss.Color("244")
	cell := func(text string, w int, style lipgloss.Style) string {
		return style.Width(w).MaxWidth(w).Render(Truncate(text, w))
	}
	right, left := lipgloss.NewStyle().Align(lipgloss.Right), lipgloss.NewStyle()

	out := []string{Brand.Render("⇅ Shared files") + Dim.Render(fmt.Sprintf("  ·  %d files", len(list))), ""}
	if len(list) == 0 {
		return strings.Join(append(out,
			"  "+Text.Render("Nothing has been shared yet."),
			"  "+Dim.Render("Share a file or folder with /upload PATH, in any chat."),
		), "\n")
	}

	out = append(out,
		"  "+cell("#", numberWidth, right.Foreground(grey))+"  "+cell("HASH", hashWidth, left.Foreground(grey))+"  "+cell("FILE", fileWidth, left.Foreground(grey))+
			"  "+cell("SIZE", sizeWidth, right.Foreground(grey))+"  "+cell("FROM", fromWidth, left.Foreground(grey))+"  "+cell("WHEN", whenWidth, left.Foreground(grey)),
		Dim.Render("  "+strings.Repeat("─", max(10, width-4))))
	for _, file := range list {
		from, style := "you", left.Foreground(lipgloss.Color("114"))
		if file.From != "" {
			from, style = "@"+file.From, left.Foreground(lipgloss.Color("111"))
		}
		out = append(out, "  "+
			cell(fmt.Sprint(file.Number), numberWidth, right.Foreground(lipgloss.Color("87")).Bold(true))+"  "+
			cell(lookup.Short(file.Hash), hashWidth, left.Foreground(grey))+"  "+
			cell(file.Name, fileWidth, Text)+"  "+
			cell(files.FormatSize(file.Size), sizeWidth, right.Foreground(grey))+"  "+
			cell(from, fromWidth, style)+"  "+
			cell(file.At.Format("02 Jan 15:04"), whenWidth, left.Foreground(grey)))
	}

	out = append(out, "", Heading.Render("GET A FILE"),
		musicHelp(width, "/download FILE [FOLDER]", "save it (default: the current folder); its MD5 is checked"),
		musicHelp(width, "/link FILE", "show its link, which holds the decryption key"),
		musicHelp(width, "/share FILE", "post it in the chat you are in"),
		"", Heading.Render("NAMING A FILE"),
		wrapped("FILE is its number, the start of its HASH (4 characters is enough), or its name.", width, Text),
		wrapped("A name with spaces goes in quotes. Files are kept on the server for a few days only.", width, Dim),
	)
	return strings.Join(out, "\n")
}
