package client

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"pipechat/internal/client/files"
)

// /open HASH|NAME downloads a shared file (or finds it again, if it was opened before) and opens it with
// the program the computer uses for that kind of file. Files are kept in the system's temporary folder, one
// folder per file, so opening the same file twice downloads it once.

// launch opens a path with the system's default program. A variable, so tests can watch it.
var launch = realLaunch

func realLaunch(path string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	case "darwin":
		command = exec.Command("open", path)
	default:
		command = exec.Command("xdg-open", path)
	}
	if err := command.Start(); err != nil {
		return err
	}
	go command.Wait() // do not leave a zombie; the program runs on its own
	return nil
}

// programTypes are files that run as programs when opened. A file someone else shared is never started
// automatically: it is saved, and the person decides.
var programTypes = map[string]bool{
	".exe": true, ".com": true, ".scr": true, ".msi": true, ".bat": true, ".cmd": true, ".ps1": true,
	".vbs": true, ".vbe": true, ".js": true, ".jse": true, ".wsf": true, ".hta": true, ".lnk": true,
	".jar": true, ".sh": true, ".bash": true, ".run": true, ".appimage": true, ".desktop": true,
	".command": true, ".app": true, ".dll": true, ".reg": true, ".py": true, ".pl": true, ".rb": true,
}

func isProgram(path string) bool {
	return programTypes[strings.ToLower(filepath.Ext(path))]
}

// openFolder is where a shared file is kept for /open.
func openFolder(file sharedFile) string {
	key := file.Hash
	if key == "" {
		key = files.ShortName(file.Name)
	}
	return filepath.Join(os.TempDir(), "pipechat-open", key)
}

func (m *Model) startOpen(current *groupState, argument string) tea.Cmd {
	argument = strings.TrimSpace(argument)
	if argument == "" {
		m.notice(current, "usage: /open HASH|NAME|NUMBER  downloads a shared file and opens it (see /files)")
		return nil
	}
	file, err := m.findShared(argument)
	if err != nil {
		m.notice(current, err.Error())
		return nil
	}
	folder := openFolder(file)
	if names, err := os.ReadDir(folder); err == nil && len(names) > 0 {
		return m.openDownloaded(current, file.Name, folder, entryNames(names))
	}
	if m.busy {
		m.notice(current, "another transfer is still running")
		return nil
	}
	if _, _, err := files.ParseLink(file.Link); err != nil {
		m.notice(current, err.Error())
		return nil
	}
	m.busy, m.transfer, m.opening = true, "downloading "+file.Name, true
	m.notice(current, "downloading "+file.Name+" to open it...")
	link, want := file.Link, file.Hash
	return func() tea.Msg { return files.Download(m.server, link, folder, file.Name, want) }
}

func entryNames(entries []os.DirEntry) []string {
	var names []string
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".pipechat-download-") {
			names = append(names, entry.Name())
		}
	}
	return names
}

// openDownloaded opens what is in folder: the file itself when there is one, otherwise the folder.
func (m *Model) openDownloaded(current *groupState, name string, folder string, entries []string) tea.Cmd {
	target := folder
	if len(entries) == 1 {
		target = filepath.Join(folder, entries[0])
	}
	if info, err := os.Stat(target); err == nil && !info.IsDir() && isProgram(target) {
		m.notice(current, fmt.Sprintf("%s is a program, so it is not started for you. It is saved at %s", name, target))
		return nil
	}
	if err := launch(target); err != nil {
		m.notice(current, fmt.Sprintf("could not open %s (%v). It is saved at %s", name, err, target))
		return nil
	}
	m.notice(current, "opened "+target)
	return nil
}
