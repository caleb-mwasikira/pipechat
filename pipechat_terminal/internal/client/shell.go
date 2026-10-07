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

	"pipechat/internal/client/ui"
)

// Shell commands. "!ls -l" (or "/sh ls -l") runs a command on this computer and shows what it printed.
// Nothing is sent to the chat. The command runs without a keyboard, so programs that ask questions or
// take over the screen (vim, top, ssh) will not work; `cd` is understood, so uploads can use short paths.

const (
	shellTimeout   = 60 * time.Second
	shellMaxOutput = 256 << 10 // bytes kept of what a command prints
	shellInline    = 12        // up to this many lines are shown in the chat; longer output opens a page
)

type shellDone struct {
	command string
	output  string
	err     error
	timeout bool
}

// shellCommand returns the command typed after "!" or "/sh ", and whether the line was one.
func shellCommand(text string) (string, bool) {
	switch {
	case strings.HasPrefix(text, "!"):
		return strings.TrimSpace(text[1:]), true
	case text == "/sh" || strings.HasPrefix(text, "/sh "):
		return strings.TrimSpace(strings.TrimPrefix(text, "/sh")), true
	}
	return "", false
}

func (m *Model) runShell(current *groupState, command string) tea.Cmd {
	if command == "" {
		m.notice(current, "usage: !COMMAND  (or /sh COMMAND) runs a command on your computer, e.g. !ls -l  ·  !cd ~/Music")
		return nil
	}
	// cd has to change this program's own folder, which a child process cannot do.
	if word, rest, _ := strings.Cut(command, " "); word == "cd" {
		return m.changeDirectory(current, strings.TrimSpace(rest))
	}
	m.notice(current, "$ "+command)
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
		return shellDone{command: command, output: output.String(), err: err, timeout: context.Err() != nil}
	}
}

func (m *Model) changeDirectory(current *groupState, argument string) tea.Cmd {
	if argument == "" || argument == "~" {
		argument = "~"
	}
	if strings.HasPrefix(argument, `"`) && strings.HasSuffix(argument, `"`) && len(argument) > 1 {
		argument = argument[1 : len(argument)-1]
	}
	if argument == "~" || strings.HasPrefix(argument, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			argument = home + argument[1:]
		}
	}
	if err := os.Chdir(argument); err != nil {
		m.notice(current, "cd: "+err.Error())
		return nil
	}
	directory, _ := os.Getwd()
	m.notice(current, "now in "+directory)
	return nil
}

func (m *Model) shellFinished(result shellDone) tea.Cmd {
	current := m.groups[m.current]
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
