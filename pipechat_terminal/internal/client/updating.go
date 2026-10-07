package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"pipechat/internal/client/files"
	"pipechat/internal/client/ui"
	"pipechat/internal/client/update"
)

// Keeping the program up to date. After logging in, the client asks the server for the newest version for this
// computer. If it is newer, a page offers to update now; saying yes downloads it, checks it, swaps it in for
// the running program, and the program starts again. The work itself is in the update package.

// executable is the file an update replaces. Tests point it somewhere harmless.
var executable = update.Executable

// Results reported back to the screen.
type updateChecked struct {
	release *update.Release
	err     error
	manual  bool // the person asked with /update
}
type updateDownloaded struct {
	release update.Release
	stage   string
	exe     string
	err     error
}
type updateInstalled struct {
	release update.Release
	exe     string
	err     error
}

// checkForUpdate asks the server whether a newer client exists.
func (m *Model) checkForUpdate(manual bool) {
	if m.server == nil {
		return
	}
	server := m.server
	go func() {
		release, err := update.Check(context.Background(), server.URL())
		server.report(updateChecked{release: release, err: err, manual: manual})
	}()
}

// handleUpdateChecked decides whether to bother the person.
func (m *Model) handleUpdateChecked(result updateChecked) tea.Cmd {
	current := m.groups[m.current]
	switch {
	case result.err != nil:
		if result.manual {
			m.notice(current, "could not check for updates: "+result.err.Error())
		}
		return nil
	case result.release == nil || !result.release.Newer():
		if result.manual {
			m.notice(current, "you have the newest version (v"+strings.TrimPrefix(update.Current, "v")+")")
		}
		return nil
	case !result.manual && m.store.SkippedVersion == result.release.Version:
		return nil // they chose to skip this one
	case m.showHelp:
		// Something else is on the screen; do not take it over. /update brings the offer back.
		m.notice(current, "a new version, v"+result.release.Version+", is available: type /update")
		return nil
	}
	release := *result.release
	m.pendingUpdate = &release
	m.pageFooter = "Y update now  ·  N not now  ·  S skip this version"
	return m.openPage("update available", func(width int) string {
		return ui.UpdatePrompt(width, update.Current, release.Version, release.Size, release.SHA256)
	})
}

// answerUpdate handles a key on the offer page. It returns whether the key was for it.
func (m *Model) answerUpdate(key string) (tea.Cmd, bool) {
	release := *m.pendingUpdate
	switch key {
	case "y", "Y", "enter":
		m.closePage()
		return m.startUpdate(release), true
	case "n", "N", "esc", "q":
		m.closePage()
		m.notice(m.groups[m.current], "update to v"+release.Version+" skipped for now; /update brings it back")
		return tea.ClearScreen, true
	case "s", "S":
		m.closePage()
		m.store.SkippedVersion = release.Version
		m.store.Save()
		m.notice(m.groups[m.current], "v"+release.Version+" will not be offered again; /update still installs it")
		return tea.ClearScreen, true
	}
	return nil, false
}

// closePage puts the chat back after a full-screen page.
func (m *Model) closePage() {
	m.showHelp, m.pendingUpdate, m.pageFooter = false, nil, ""
}

// startUpdate downloads the new program next to the running one.
func (m *Model) startUpdate(release update.Release) tea.Cmd {
	current := m.groups[m.current]
	if m.busy {
		m.notice(current, "another transfer is still running; try /update when it is done")
		return tea.ClearScreen
	}
	exe, err := executable()
	if err != nil {
		m.notice(current, "cannot find the running program: "+err.Error())
		return tea.ClearScreen
	}
	m.busy, m.transfer = true, "updating"
	m.notice(current, "downloading v"+release.Version+"...")
	server, name := m.server, "pipechat v"+release.Version
	stage := update.StagePath(exe)
	return tea.Batch(tea.ClearScreen, func() tea.Msg {
		var last time.Time
		err := update.Download(context.Background(), server.URL(), release, stage, func(done, total int64) {
			if time.Since(last) < 150*time.Millisecond && done != total {
				return // the screen needs a few updates a second, not one per chunk
			}
			last = time.Now()
			server.emit(files.Progress{Verb: "downloading", Name: name, Done: done, Total: total})
		})
		return updateDownloaded{release: release, stage: stage, exe: exe, err: err}
	})
}

// handleUpdateDownloaded swaps the new program in.
func (m *Model) handleUpdateDownloaded(result updateDownloaded) tea.Cmd {
	m.busy, m.transfer, m.progress = false, "", nil
	m.layout()
	current := m.groups[m.current]
	if result.err != nil {
		m.notice(current, "update failed: "+result.err.Error()+" (the program was not changed)")
		return nil
	}
	return func() tea.Msg {
		return updateInstalled{release: result.release, exe: result.exe, err: update.Replace(result.stage, result.exe)}
	}
}

// handleUpdateInstalled ends the screen so main can start the new program.
func (m *Model) handleUpdateInstalled(result updateInstalled) tea.Cmd {
	if result.err != nil {
		m.notice(m.groups[m.current], "update failed: "+result.err.Error()+" (the program was not changed)")
		return nil
	}
	m.stopMusic()
	m.restartExe = result.exe
	return tea.Quit
}

// RestartExe is the program to start again once the screen has closed, after an update; empty when the
// person simply quit.
func (m *Model) RestartExe() string { return m.restartExe }

// updateCommand handles /update: check now, and offer the update even if it was skipped.
func (m *Model) updateCommand(current *groupState) {
	if update.Current == "dev" {
		m.notice(current, "this is a development build (no version), so it never updates itself")
		return
	}
	m.notice(current, fmt.Sprintf("you have v%s; checking for a newer one...", strings.TrimPrefix(update.Current, "v")))
	m.checkForUpdate(true)
}
