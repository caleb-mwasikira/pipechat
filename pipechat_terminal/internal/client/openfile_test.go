package client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenLaunchesAFileThatIsAlreadyDownloaded(t *testing.T) {
	m := loggedInModel(t)
	file := sharedFile{Name: "photo.jpg", fileRecord: fileRecord{Hash: "0123456789abcdef0123456789abcdef"}}
	m.vault.Files = map[string]fileRecord{"photo.jpg": file.fileRecord}
	folder := openFolder(file)
	t.Cleanup(func() { os.RemoveAll(folder) })
	os.MkdirAll(folder, 0o755)
	os.WriteFile(filepath.Join(folder, "photo.jpg"), []byte("jpeg"), 0o644)

	var opened string
	launch = func(path string) error { opened = path; return nil }
	t.Cleanup(func() { launch = realLaunch })

	m.startOpen(m.groups[m.current], "0123")
	if opened != filepath.Join(folder, "photo.jpg") {
		t.Fatalf("opened %q", opened)
	}
	opened = ""
	m.startOpen(m.groups[m.current], "photo.jpg")
	if opened == "" {
		t.Fatal("a name works too")
	}
}

func TestOpenNeverStartsAProgram(t *testing.T) {
	m := loggedInModel(t)
	folder := t.TempDir()
	os.WriteFile(filepath.Join(folder, "setup.exe"), []byte("MZ"), 0o755)
	launched := false
	launch = func(string) error { launched = true; return nil }
	t.Cleanup(func() { launch = realLaunch })

	m.openDownloaded(m.groups[m.current], "setup.exe", folder, []string{"setup.exe"})
	if launched {
		t.Fatal("programs must not be started automatically")
	}
	if !strings.Contains(entriesText(m.groups[m.current]), "not started") {
		t.Fatal("the person is told where it was saved")
	}
}
