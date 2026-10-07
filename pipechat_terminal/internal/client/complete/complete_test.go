package complete

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tree(t *testing.T) string {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "docs"), 0o755)
	os.MkdirAll(filepath.Join(root, "My Music"), 0o755)
	os.WriteFile(filepath.Join(root, "readme.md"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(root, "report.pdf"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(root, ".hidden"), []byte("x"), 0o644)
	return root
}

func TestCommandsStartWithHelp(t *testing.T) {
	if Commands[0] != "/help" {
		t.Fatalf("a bare / should suggest %q first", Commands[0])
	}
	seen := map[string]bool{}
	for _, command := range Commands {
		if !strings.HasPrefix(command, "/") || seen[command] {
			t.Errorf("bad or repeated command %q", command)
		}
		seen[command] = true
	}
}

func TestPaths(t *testing.T) {
	root := tree(t)
	got := Paths("/upload " + root + "/re")
	want := []string{"/upload " + root + "/readme.md", "/upload " + root + "/report.pdf"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v want %v", got, want)
	}
	if got := Paths("/upload " + root + "/"); got[0] != "/upload "+root+"/My Music/" && !strings.Contains(strings.Join(got, "|"), "docs/") {
		t.Fatalf("folders come first: %v", got)
	}
	for _, suggestion := range Paths("/upload " + root + "/") {
		if strings.Contains(suggestion, ".hidden") {
			t.Fatal("hidden files are not offered")
		}
	}
	if got := Paths("/upload " + root + "/.h"); len(got) != 1 {
		t.Fatalf("hidden when asked for: %v", got)
	}
	if got := Paths(`/upload "` + root + "/My"); len(got) != 1 || !strings.HasSuffix(got[0], `My Music/`) {
		t.Fatalf("a quoted folder: %v", got)
	}
}

func TestPathsOnlyWhereAPathGoes(t *testing.T) {
	root := tree(t)
	for _, line := range []string{
		"/play " + root + "/",                    // not a file command
		"/upload -name " + root + "/re",          // a short name, not a path
		"/upload " + root + "/readme.md " + root, // one path is enough
		"/download " + root + "/",                // the first /download word is a name or link
		"hello",
	} {
		if got := Paths(line); len(got) != 0 {
			t.Errorf("%q should have no path suggestions: %v", line, got)
		}
	}
	if got := Paths("/download notes " + root + "/"); len(got) == 0 {
		t.Error("the folder to save into should complete")
	} else {
		for _, suggestion := range got {
			if !strings.HasSuffix(suggestion, "/") {
				t.Errorf("only folders fit /download: %s", suggestion)
			}
		}
	}
}
