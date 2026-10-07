package client

import (
	"os"
	"strings"
	"testing"

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
