package client

import (
	"fmt"
	"os"
	"path/filepath"
	"pipechat/internal/client/ui"

	tea "github.com/charmbracelet/bubbletea"

	"pipechat/internal/client/files"
)

// Before anything is sent, /upload opens a page showing what is about to be shared and with whom,
// and waits for a yes. The page is made from what is on disk; nothing leaves the computer yet.

const previewTree = 16 // entries of a folder listed

// pendingUpload is an upload waiting for the person to say yes on the preview page.
type pendingUpload struct {
	target   *groupState
	argument string
}

// uploadArguments splits "/upload" arguments into the path and the short name.
func uploadArguments(argument string) (source string, name string, ok bool) {
	arguments := files.SplitArguments(argument)
	for index := 0; index < len(arguments); index++ {
		switch {
		case (arguments[index] == "-name" || arguments[index] == "--name") && index+1 < len(arguments):
			index++
			name = arguments[index]
		case source == "":
			source = arguments[index]
		default:
			return "", "", false
		}
	}
	return source, name, source != ""
}

// previewUpload handles /upload: show the preview page, and upload only after a yes.
func (m *Model) previewUpload(current *groupState, argument string) tea.Cmd {
	source, name, ok := uploadArguments(argument)
	if !ok {
		m.notice(current, "usage: /upload PATH [-name SHORTNAME]  (quote paths with spaces; Tab completes a path)")
		return nil
	}
	path := files.ExpandHome(source)
	info, err := os.Stat(path)
	if err != nil {
		m.notice(current, "cannot read "+source+": "+err.Error())
		return nil
	}
	if current.secret == nil {
		m.notice(current, "open a group or direct chat first, then /upload")
		return nil
	}
	if name == "" {
		absolute, _ := filepath.Abs(path)
		name = files.ShortName(filepath.Base(absolute))
	}
	if !files.ShortNamePattern.MatchString(name) {
		m.notice(current, "short name must be 1-40 characters: letters, digits, . _ -")
		return nil
	}

	where := current.name()
	if current.direct {
		where = "a private chat with @" + current.peer
	} else if len(m.peersOnline[current.secret.GIDHex()]) > 0 {
		where += fmt.Sprintf(" (%d online now)", len(m.peersOnline[current.secret.GIDHex()]))
	}
	content := func(width int) string { return ui.UploadPreviewPage(width, path, info, name, where) }
	m.pendingSend = &pendingUpload{target: current, argument: argument}
	m.pageFooter = "Enter or y to upload  ·  Esc or n to cancel  ·  ↑/↓ scroll"
	return m.openPage("upload "+name, content)
}
