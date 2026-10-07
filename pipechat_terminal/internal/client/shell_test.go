package client

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestFolderListingShowsWhileTypingAPath(t *testing.T) {
	root := makeTree(t)
	m := loggedInModel(t)

	typeInto(m, "/upload "+root+"/")
	rows := strings.Join(m.browsePanel(), "\n")
	for _, want := range []string{"docs/", "readme.md", "report.pdf"} {
		if !strings.Contains(ansi.Strip(rows), want) {
			t.Fatalf("the listing should show %q:\n%s", want, rows)
		}
	}
	if strings.Contains(rows, ".hidden") {
		t.Fatal("hidden files stay out of the listing")
	}
	typeInto(m, "hello")
	if m.browsePanel() != nil {
		t.Fatal("no listing for ordinary messages")
	}
}

func TestShellCommandsRunHereAndStayOutOfTheChat(t *testing.T) {
	m := loggedInModel(t)
	root := makeTree(t)
	start, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(start) })

	if cmd := m.runShell(m.groups[m.current], "cd "+root); cmd != nil {
		t.Fatal("cd needs no process")
	}
	if now, _ := os.Getwd(); !strings.HasSuffix(now, root[strings.LastIndex(root, "/"):]) {
		t.Fatalf("cd should move the client: %s", now)
	}

	run := m.runShell(m.groups[m.current], "ls")
	done, ok := run().(shellDone)
	if !ok || done.err != nil || !strings.Contains(done.output, "report.pdf") {
		t.Fatalf("ls output: %+v", done)
	}
	m.shellFinished(done)
	var seen string
	for _, line := range m.groups[m.current].entries {
		seen += line.text + "\n"
	}
	if !strings.Contains(seen, "report.pdf") {
		t.Fatalf("short output goes into the chat view:\n%s", seen)
	}

	long := m.runShell(m.groups[m.current], "seq 1 50")().(shellDone)
	m.shellFinished(long)
	if !m.showHelp || !strings.HasPrefix(m.pageTitle, "$ seq") {
		t.Fatalf("long output opens a page: %v %q", m.showHelp, m.pageTitle)
	}
}

func shellText(m *Model) string { return ansi.Strip(strings.Join(m.shellLog, "\n")) }

func TestShellScreenRunsPlainLinesAsCommands(t *testing.T) {
	m := loggedInModel(t)
	m.updateChat(tea.KeyMsg{Type: tea.KeyCtrlO})
	if !m.shellMode || !strings.HasSuffix(m.input.Prompt, "$ ") || m.input.Placeholder != shellPlaceholder {
		t.Fatalf("Ctrl+O should open the shell: %v %q", m.shellMode, m.input.Prompt)
	}
	if m.mouseNow() {
		t.Fatal("the shell lets go of the mouse, so its text can be copied")
	}
	if !strings.Contains(ansi.Strip(m.View()), "back to chat") {
		t.Fatal("the shell screen says how to leave")
	}
	typeInto(m, "echo hello-from-shell")
	_, command := m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("the line should run as a command")
	}
	m.Update(command())
	if text := shellText(m); !strings.Contains(text, "$ echo hello-from-shell") || !strings.Contains(text, "hello-from-shell\n") {
		t.Fatalf("the record shows the command and its output:\n%s", text)
	}

	m.updateChat(tea.KeyMsg{Type: tea.KeyUp})
	if m.input.Value() != "echo hello-from-shell" {
		t.Fatalf("Up brings back the last command: %q", m.input.Value())
	}
	m.input.Reset()

	m.updateChat(tea.KeyMsg{Type: tea.KeyCtrlL})
	if len(m.shellLog) != 0 {
		t.Fatal("Ctrl+L clears the record")
	}
	typeInto(m, "echo again")
	_, command = m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
	m.Update(command())
	typeInto(m, "clear")
	m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.shellLog) != 0 {
		t.Fatal("typing clear clears the record")
	}
	if view := ansi.Strip(m.View()); strings.Index(view, "back to chat") > strings.Index(view, "\n") {
		t.Fatal("the hint is on the top line")
	}

	typeInto(m, "exit")
	m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
	if m.shellMode || m.input.Prompt != "> " || m.input.Placeholder != chatPlaceholder {
		t.Fatal("exit should go back to chatting")
	}
}

func TestTabCompletesInTheShell(t *testing.T) {
	root := makeTree(t)
	m := loggedInModel(t)
	m.setShellMode(true)

	typeInto(m, "cat "+root+"/rep")
	m.updateChat(tea.KeyMsg{Type: tea.KeyTab})
	if m.input.Value() != "cat "+root+"/report.pdf " {
		t.Fatalf("one match completes the word: %q", m.input.Value())
	}
	typeInto(m, "ls "+root+"/re")
	m.updateChat(tea.KeyMsg{Type: tea.KeyTab})
	if m.input.Value() != "ls "+root+"/re" || !strings.Contains(shellText(m), "readme.md  report.pdf") {
		t.Fatalf("several matches are listed in the record: %q\n%s", m.input.Value(), shellText(m))
	}
	typeInto(m, "cd "+root+"/My\\ Mu")
	m.updateChat(tea.KeyMsg{Type: tea.KeyTab})
	if m.input.Value() != "cd "+root+"/My\\ Music/" {
		t.Fatalf("names with spaces are escaped: %q", m.input.Value())
	}
}

func TestDeleteCommandsExplainThemselvesAndRefuseUnknownFiles(t *testing.T) {
	m := loggedInModel(t)
	current := m.groups[m.current]
	m.deleteCommand(current, "")
	m.deleteCommand(current, "nothing-here")
	m.deleteMusicCommand(current, "")
	text := entriesText(current)
	for _, want := range []string{"usage: /delete NAME", "no shared files yet", "usage: /delete-music"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
}
