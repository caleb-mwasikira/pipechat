package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"pipechat/internal/client/files"
)

// previewTree is how many entries of a folder are listed.
const previewTree = 16

// UploadPreviewPage describes what is about to be shared.
func UploadPreviewPage(width int, path string, info os.FileInfo, name string, where string) string {
	label := lipgloss.NewStyle().Width(16).Foreground(lipgloss.Color("244"))
	row := func(key string, value string) string { return "  " + label.Render(key) + Text.Render(value) }
	absolute, _ := filepath.Abs(path)

	var out []string
	out = append(out, Brand.Render("Ready to upload?"), "")
	out = append(out, row("Share as", name), row("Sharing in", where), row("From", absolute))

	if info.IsDir() {
		out = append(out, folderPreview(absolute, label, width)...)
	} else {
		out = append(out, filePreview(absolute, info, label, width)...)
	}
	out = append(out, "",
		Dim.Render("  The "+kindWord(info)+" is packed and encrypted on this computer before it leaves it. The link, which holds the"),
		Dim.Render("  key, goes to the chat as a message. The server's size limit and the file's expiry apply."))
	return strings.Join(out, "\n")
}

func kindWord(info os.FileInfo) string {
	if info.IsDir() {
		return "folder"
	}
	return "file"
}

func filePreview(path string, info os.FileInfo, label lipgloss.Style, width int) []string {
	row := func(key string, value string) string { return "  " + label.Render(key) + Text.Render(value) }
	kind, preview := files.Describe(path)
	out := []string{row("Type", kind), row("Size", files.FormatSize(info.Size())), row("Modified", info.ModTime().Format("2 Jan 2006 15:04"))}
	if len(preview) > 0 {
		out = append(out, "", Heading.Render("PREVIEW"))
		box := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
		for _, line := range preview {
			out = append(out, "  "+Dim.Render("│ ")+box.Render(Clip(line, max(10, width-8))))
		}
	}
	return out
}

func Clip(line string, width int) string {
	if len([]rune(line)) <= width {
		return line
	}
	return string([]rune(line)[:width-1]) + "…"
}

func folderPreview(root string, label lipgloss.Style, width int) []string {
	info := files.Summarize(root)
	more := ""
	if info.Truncated {
		more = " or more"
	}
	row := func(key string, value string) string { return "  " + label.Render(key) + Text.Render(value) }
	out := []string{row("Type", "folder"), row("Contents", fmt.Sprintf("%d files%s in %d folders%s, %s%s", info.Files, more, info.Folders, more, files.FormatSize(info.Total), more))}
	if len(info.Entries) == 0 {
		return append(out, "", "  "+Dim.Render("The folder is empty."))
	}
	out = append(out, "", Heading.Render("CONTENTS"))
	for index, item := range info.Entries {
		if index >= previewTree {
			out = append(out, "  "+Dim.Render(fmt.Sprintf("… and %d more", len(info.Entries)-previewTree)))
			break
		}
		size := ""
		if !item.Dir {
			size = Dim.Render("  " + files.FormatSize(item.Size))
		}
		out = append(out, "  "+Dim.Render("│ ")+Text.Render(Clip(item.Path, max(10, width-24)))+size)
	}
	return out
}
