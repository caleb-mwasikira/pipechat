package client

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"pipechat/internal/client/complete"
	"pipechat/internal/client/ui"
)

// Shell commands. "/sh ls -l" runs one command on this computer and shows what it printed in the chat.
// "/shell" (or Ctrl+O) opens a full-screen shell, like the help pages: a scrolling record of everything
// run, and a prompt at the bottom. The mouse is let go of there, so the terminal can select and copy any
// of it. Nothing is ever sent to the chat. Commands run without a keyboard, so programs that ask
// questions or take over the screen (vim, top, ssh) will not work; `cd` is understood, so uploads can
// use short paths.

// What the empty input says, so the way to the other mode is always in view.
const (
	chatPlaceholder  = "message or /command  ·  Ctrl+O: shell mode"
	shellPlaceholder = "shell command  ·  Ctrl+O or exit: back to chat"
)

const (
	shellTimeout   = 60 * time.Second
	shellMaxOutput = 256 << 10 // bytes kept of what a command prints
	shellInline    = 12        // in the chat, up to this many lines are shown; longer output opens a page
	shellKeep      = 5000      // lines of the shell's record that are kept
)

type shellDone struct {
	command string
	output  string
	err     error
	timeout bool
	inShell bool // started from the shell screen: the answer goes there
}

// shellCommand returns the command typed after "/sh ", and whether the line was one.
func shellCommand(text string) (string, bool) {
	if text == "/sh" || strings.HasPrefix(text, "/sh ") {
		return strings.TrimSpace(strings.TrimPrefix(text, "/sh")), true
	}
	return "", false
}

// execute runs a command line on this computer, in the background.
func execute(command string, inShell bool) tea.Cmd {
	return func() tea.Msg {
		context, cancel := context.WithTimeout(context.Background(), shellTimeout)
		defer cancel()
		var process *exec.Cmd
		if runtime.GOOS == "windows" {
			process = exec.CommandContext(context, "cmd", "/C", command)
		} else {
			process = exec.CommandContext(context, "sh", "-c", command)
		}
		var output limitedBuffer
		process.Stdout, process.Stderr = &output, &output
		err := process.Run()
		return shellDone{command: command, output: output.String(), err: err, timeout: context.Err() != nil, inShell: inShell}
	}
}

// changeDirectory is `cd`: it has to move this program itself, which a child process cannot do.
func changeDirectory(argument string) (string, error) {
	if len(argument) > 1 && strings.HasPrefix(argument, `"`) && strings.HasSuffix(argument, `"`) {
		argument = argument[1 : len(argument)-1]
	}
	if argument == "" || argument == "~" || strings.HasPrefix(argument, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			argument = home + strings.TrimPrefix(argument, "~")
		}
	}
	if err := os.Chdir(argument); err != nil {
		return "", err
	}
	directory, _ := os.Getwd()
	return directory, nil
}

func isCd(command string) (string, bool) {
	word, rest, _ := strings.Cut(command, " ")
	return strings.TrimSpace(rest), word == "cd"
}

// runShell is /sh COMMAND: one command, answered in the chat.
func (m *Model) runShell(current *groupState, command string) tea.Cmd {
	if command == "" {
		m.notice(current, "usage: /sh COMMAND runs one command on your computer, e.g. /sh ls -l  ·  /shell opens a shell of its own")
		return nil
	}
	if argument, ok := isCd(command); ok {
		if directory, err := changeDirectory(argument); err != nil {
			m.notice(current, "cd: "+err.Error())
		} else {
			m.notice(current, "now in "+directory)
		}
		return nil
	}
	m.notice(current, "$ "+command)
	return execute(command, false)
}

// shellFinished shows what a command printed: in the shell screen if it was started there, otherwise in
// the chat (short output) or on a page of its own (long output).
func (m *Model) shellFinished(result shellDone) tea.Cmd {
	text := strings.TrimRight(strings.ReplaceAll(ansi.Strip(result.output), "\r\n", "\n"), "\n")
	var footer string
	switch {
	case result.timeout:
		footer = fmt.Sprintf("stopped after %s", shellTimeout)
	case result.err != nil:
		footer = "failed: " + result.err.Error()
	}
	lines := strings.Split(text, "\n")
	if text == "" {
		lines = nil
	}

	if result.inShell {
		m.shellAppend(lines...)
		if footer != "" {
			m.shellAppend(ui.Dim.Render("(" + footer + ")"))
		}
		return nil
	}
	current := m.groups[m.current]
	if len(lines) <= shellInline {
		for _, line := range lines {
			m.notice(current, "  "+line)
		}
		if footer != "" {
			m.notice(current, "  ("+footer+")")
		}
		return nil
	}
	content := func(width int) string {
		body := lipgloss.NewStyle().Width(max(20, width-2)).Render(text)
		if footer != "" {
			body += "\n\n" + ui.Dim.Render(footer)
		}
		return body
	}
	m.notice(current, fmt.Sprintf("  (%d lines, opened on its own page)", len(lines)))
	command := m.openPage("$ "+result.command, content)
	m.pageFooter = "Esc to close  ·  ↑/↓ scroll  ·  / to search"
	return command
}

// limitedBuffer keeps the first shellMaxOutput bytes and drops the rest, so a runaway command cannot fill memory.
type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(data []byte) (int, error) {
	if room := shellMaxOutput - b.Len(); room > 0 {
		b.Buffer.Write(data[:min(room, len(data))])
	}
	return len(data), nil
}

// --- the shell screen ---

// shellPrompt is the folder (with the home folder as ~) and a dollar sign.
func shellPrompt() string {
	directory, err := os.Getwd()
	if err != nil {
		return "$ "
	}
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(directory, home) {
		directory = "~" + strings.TrimPrefix(directory, home)
	}
	return directory + " $ "
}

// setShellMode switches between the chat and the shell screen.
func (m *Model) setShellMode(on bool) tea.Cmd {
	if m.shellMode == on {
		return nil
	}
	m.shellMode = on
	m.input.Reset()
	if on {
		m.input.Prompt, m.input.Placeholder = shellPrompt(), shellPlaceholder
		m.layoutShell()
	} else {
		m.input.Prompt, m.input.Placeholder = "> ", chatPlaceholder
	}
	m.refreshSuggestions()
	return tea.ClearScreen
}

// layoutShell sizes the record to the window and draws it, scrolled to the end.
func (m *Model) layoutShell() {
	m.shellScreen.Width, m.shellScreen.Height = m.width, max(3, m.height-2)
	wrap := lipgloss.NewStyle().Width(max(10, m.width))
	m.shellScreen.SetContent(wrap.Render(strings.Join(m.shellLog, "\n")))
	m.shellScreen.GotoBottom()
}

func (m *Model) shellAppend(lines ...string) {
	m.shellLog = append(m.shellLog, lines...)
	if extra := len(m.shellLog) - shellKeep; extra > 0 {
		m.shellLog = m.shellLog[extra:]
	}
	m.layoutShell()
}

// shellKey handles a key on the shell screen.
func (m *Model) shellKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+o":
		return m, m.setShellMode(false)
	case "pgup", "pgdown":
		var command tea.Cmd
		m.shellScreen, command = m.shellScreen.Update(key)
		return m, command
	case "esc":
		m.shellScreen.GotoBottom()
		return m, nil
	case "ctrl+l":
		m.shellClear()
		return m, nil
	case "up", "down":
		m.shellHistoryStep(key.String() == "up")
		return m, nil
	case "tab":
		m.completeShell()
		return m, nil
	case "enter":
		text := strings.TrimSpace(m.input.Value())
		m.input.Reset()
		m.shellAt = len(m.shellHistory)
		return m, m.shellRun(text)
	}
	var command tea.Cmd
	m.input, command = m.input.Update(key)
	return m, command
}

// shellRun is Enter on the shell screen.
func (m *Model) shellRun(text string) tea.Cmd {
	if text == "exit" || text == "/chat" {
		return m.setShellMode(false)
	}
	if text == "clear" {
		m.shellClear()
		return nil
	}
	m.shellAppend(ui.Key.Render(m.input.Prompt) + text)
	if text == "" {
		return nil
	}
	m.shellHistory = append(m.shellHistory, text)
	m.shellAt = len(m.shellHistory)
	if argument, ok := isCd(text); ok {
		if _, err := changeDirectory(argument); err != nil {
			m.shellAppend("cd: " + err.Error())
		}
		m.input.Prompt = shellPrompt()
		return nil
	}
	return execute(text, true)
}

// shellClear empties the record, like `clear` in a terminal.
func (m *Model) shellClear() {
	m.shellLog = nil
	m.layoutShell()
}

// shellHistoryStep puts an earlier (or later) command into the input.
func (m *Model) shellHistoryStep(older bool) {
	if len(m.shellHistory) == 0 {
		return
	}
	if older {
		m.shellAt = max(0, m.shellAt-1)
	} else {
		m.shellAt = min(len(m.shellHistory), m.shellAt+1)
	}
	line := ""
	if m.shellAt < len(m.shellHistory) {
		line = m.shellHistory[m.shellAt]
	}
	m.input.SetValue(line)
	m.input.CursorEnd()
}

// completeShell is Tab: it finishes the last word of the line, and lists the choices when there are several.
func (m *Model) completeShell() {
	line := m.input.Value()
	completed, options := complete.Shell(line)
	if completed != line {
		m.input.SetValue(completed)
		m.input.CursorEnd()
	}
	if len(options) > 1 {
		m.shellAppend(ui.Key.Render(m.input.Prompt) + line)
		names := make([]string, 0, len(options))
		for _, option := range options {
			trimmed := strings.TrimSuffix(option, "/")
			name := trimmed[strings.LastIndexAny(trimmed, `/\`)+1:]
			if strings.HasSuffix(option, "/") {
				name += "/"
			}
			names = append(names, name)
		}
		listing := strings.Join(names[:min(len(names), 200)], "  ")
		if len(options) > 200 {
			listing += fmt.Sprintf("  … %d more", len(options)-200)
		}
		m.shellAppend(ui.Dim.Render(listing))
	}
}

// shellView draws the shell screen.
func (m *Model) shellView() string {
	directory, _ := os.Getwd()
	title := ui.Title.Render("Pipechat") + ui.Dim.Render("  shell  "+directory+"  ·  ") + ui.Key.Render("exit or Ctrl+O: back to chat")
	return title + "\n" + m.shellScreen.View() + "\n" + m.input.View()
}
