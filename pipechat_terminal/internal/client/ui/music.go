package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"pipechat/internal/client/files"
	"pipechat/internal/client/lookup"
	"pipechat/internal/client/music"
)

// MusicLibrary draws the shared music library the way the help guide is drawn: a heading, aligned columns,
// and the commands that go with it. playing is the number of the song coming out of the speakers, or 0.
func MusicLibrary(width int, tracks []music.Track, playing int64, modes string) string {
	const (
		numberWidth = 5
		hashWidth   = 8
		sizeWidth   = 9
		byWidth     = 14
		gaps        = 4 * 2 // two spaces between each pair of the five columns
	)
	songWidth := max(16, width-4-numberWidth-hashWidth-sizeWidth-byWidth-gaps)
	cell := func(text string, w int, style lipgloss.Style) string {
		return style.Width(w).MaxWidth(w).Render(Truncate(text, w))
	}
	right := lipgloss.NewStyle().Align(lipgloss.Right)
	left := lipgloss.NewStyle()

	var out []string
	out = append(out, Brand.Render("♪ Music library")+Dim.Render(fmt.Sprintf("  ·  %d songs", len(tracks))+modes), "")
	if len(tracks) == 0 {
		out = append(out,
			"  "+Text.Render("The library is empty."),
			"  "+Dim.Render("Upload songs on the website, under Music, and they appear here."))
		return strings.Join(out, "\n")
	}

	header := "  " + cell("#", numberWidth, right.Foreground(lipgloss.Color("244"))) + "  " + cell("HASH", hashWidth, left.Foreground(lipgloss.Color("244"))) +
		"  " + cell("SONG", songWidth, left.Foreground(lipgloss.Color("244"))) + "  " + cell("SIZE", sizeWidth, right.Foreground(lipgloss.Color("244"))) +
		"  " + cell("ADDED BY", byWidth, left.Foreground(lipgloss.Color("244")))
	out = append(out, header, Dim.Render("  "+strings.Repeat("─", max(10, width-4))))

	for _, track := range tracks {
		name := music.FileName(track)
		by := ""
		if track.Uploader != "" {
			by = "@" + track.Uploader
		}
		marker, base := "  ", Text
		if track.ID == playing {
			marker, base = Brand.Render("♪ "), Active
		}
		row := marker + cell(fmt.Sprint(track.ID), numberWidth-2, right.Foreground(lipgloss.Color("87")).Bold(true)) + "  " +
			cell(lookup.Short(track.MD5), hashWidth, left.Foreground(lipgloss.Color("244"))) + "  " +
			cell(name, songWidth, base) + "  " +
			cell(files.FormatSize(track.Size), sizeWidth, right.Foreground(lipgloss.Color("244"))) + "  " +
			cell(by, byWidth, left.Foreground(lipgloss.Color("111")))
		out = append(out, row)
	}

	out = append(out, "", Heading.Render("PLAY"),
		musicHelp(width, "/play", "the whole library, one song after another"),
		musicHelp(width, "/play SONG", "start from that song, then carry on"),
		musicHelp(width, "/next · /stop", "skip to the next song · stop the music"),
		musicHelp(width, "/shuffle [on|off]", "play in random order"),
		musicHelp(width, "/repeat off|one|all", "repeat nothing, one song, or the whole library"),
		"", Heading.Render("KEEP"),
		musicHelp(width, "/save SONG [FOLDER]", "save one song as a file (default: your Music folder)"),
		musicHelp(width, "/save all [FOLDER]", "save the whole library"),
		"", Heading.Render("NAMING A SONG"),
		wrapped("SONG is its number, the start of its HASH (4 characters is enough), or its name.", width, Text),
		wrapped("A name with spaces goes in quotes: /play \"The Tests - Song 2\"", width, Dim),
	)
	return strings.Join(out, "\n")
}

// musicHelp is a command and what it does, in two columns; the second wraps to fit the width.
func musicHelp(width int, command string, what string) string {
	const commandWidth = 26
	description := lipgloss.NewStyle().Width(max(16, width-4-commandWidth)).Render(Text.Render(what))
	return "  " + lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(commandWidth).Render(Command.Render(command)), description)
}

// wrapped is a paragraph indented by two spaces that fits the width.
func wrapped(text string, width int, style lipgloss.Style) string {
	return "  " + strings.ReplaceAll(lipgloss.NewStyle().Width(max(20, width-4)).Render(style.Render(text)), "\n", "\n  ")
}
