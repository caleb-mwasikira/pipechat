package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// aProgram is bytes that pass for a program on the system running the test.
func aProgram(payload string) []byte {
	switch runtime.GOOS {
	case "windows":
		return append([]byte("MZ\x90\x00"), payload...)
	case "darwin":
		return append([]byte("\xcf\xfa\xed\xfe"), payload...)
	}
	return append([]byte("\x7fELF"), payload...)
}

func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func withCurrent(t *testing.T, version string) {
	previous := Current
	Current = version
	t.Cleanup(func() { Current = previous })
}

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		current, offered string
		want             bool
	}{
		{"1.0.0", "1.1.0", true},
		{"1.1.0", "1.1.0", false},
		{"1.2.0", "1.1.0", false}, // never go back
		{"1.9.0", "1.10.0", true},
		{"1.1.0-beta.1", "1.1.0", true},
		{"1.1.0", "1.1.1-beta.1", true},
		{"dev", "9.9.9", false}, // a build made from the source is left alone
		{"1.0.0", "garbage", false},
	} {
		withCurrent(t, c.current)
		if got := (Release{Version: c.offered}).Newer(); got != c.want {
			t.Errorf("running %s, offered %s: Newer = %v, want %v", c.current, c.offered, got, c.want)
		}
	}
}

func TestCheckAsksForThisSystem(t *testing.T) {
	var asked string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.RawQuery
		fmt.Fprint(w, `{"version":"1.2.0","url":"/api/update/7","size":10,"sha256":"abc","md5":"def"}`)
	}))
	defer server.Close()
	osName, arch, ok := Platform()
	if !ok {
		t.Skip("no downloads for this system")
	}
	release, err := Check(context.Background(), server.URL)
	if err != nil || release == nil || release.Version != "1.2.0" || release.URL != "/api/update/7" {
		t.Fatalf("check: %+v %v", release, err)
	}
	if asked != "os="+osName+"&arch="+arch {
		t.Fatalf("asked %q", asked)
	}
}

func TestCheckWhenThereIsNothingOrSomethingOdd(t *testing.T) {
	answers := map[string]struct {
		status int
		body   string
	}{
		"/none":      {404, `{"error":"no client"}`},
		"/broken":    {200, `not json`},
		"/odd":       {200, `{"version":"hello","url":"/api/update/1"}`},
		"/elsewhere": {200, `{"version":"1.0.0","url":"https://evil.example/x"}`}, // only this server's updates
		"/down":      {500, ``},
	}
	for name, answer := range answers {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(answer.status)
			fmt.Fprint(w, answer.body)
		}))
		release, err := Check(context.Background(), server.URL)
		server.Close()
		switch name {
		case "/none":
			if release != nil || err != nil {
				t.Errorf("no client published is not an error: %+v %v", release, err)
			}
		default:
			if release != nil || err == nil {
				t.Errorf("%s should be refused: %+v %v", name, release, err)
			}
		}
	}
	if _, err := Check(context.Background(), "http://127.0.0.1:1"); err == nil {
		t.Error("an unreachable server is an error")
	}
}

func serveFile(t *testing.T, data []byte) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/update/1" {
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestDownloadChecksAndKeepsOnlyAGoodProgram(t *testing.T) {
	data := aProgram(strings.Repeat("x", 5000))
	server := serveFile(t, data)
	good := Release{Version: "1.2.0", URL: "/api/update/1", Size: int64(len(data)), SHA256: checksum(data)}

	stage := filepath.Join(t.TempDir(), "pipechat.new")
	var last, total int64
	if err := Download(context.Background(), server.URL, good, stage, func(done, all int64) { last, total = done, all }); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(stage); !bytes.Equal(got, data) {
		t.Fatal("the file on disk is not what was served")
	}
	if last != int64(len(data)) || total != int64(len(data)) {
		t.Fatalf("progress ended at %d of %d", last, total)
	}
	if info, _ := os.Stat(stage); runtime.GOOS != "windows" && info.Mode().Perm()&0o100 == 0 {
		t.Fatal("the new program must be runnable")
	}

	for name, bad := range map[string]Release{
		"wrong checksum": {URL: good.URL, Size: good.Size, SHA256: strings.Repeat("0", 64)},
		"wrong size":     {URL: good.URL, Size: good.Size + 1, SHA256: good.SHA256},
		"no such file":   {URL: "/api/update/2", Size: good.Size, SHA256: good.SHA256},
	} {
		stage := filepath.Join(t.TempDir(), "pipechat.new")
		if err := Download(context.Background(), server.URL, bad, stage, nil); err == nil {
			t.Errorf("%s was accepted", name)
		}
		if _, err := os.Stat(stage); err == nil {
			t.Errorf("%s left a file behind", name)
		}
	}
}

func TestDownloadRefusesSomethingThatIsNotAProgram(t *testing.T) {
	page := []byte("<html>Oops, an error page</html>")
	server := serveFile(t, page)
	stage := filepath.Join(t.TempDir(), "pipechat.new")
	err := Download(context.Background(), server.URL, Release{URL: "/api/update/1", Size: int64(len(page)), SHA256: checksum(page)}, stage, nil)
	if err == nil || !strings.Contains(err.Error(), "not a program") {
		t.Fatalf("an error page must not become the program: %v", err)
	}
	if _, statErr := os.Stat(stage); statErr == nil {
		t.Fatal("it must not stay on disk")
	}
}

func TestDownloadSaysWhenItCannotWriteBesideTheProgram(t *testing.T) {
	data := aProgram("x")
	server := serveFile(t, data)
	err := Download(context.Background(), server.URL, Release{URL: "/api/update/1", Size: int64(len(data))}, filepath.Join(t.TempDir(), "missing-folder", "pipechat.new"), nil)
	if err == nil || !strings.Contains(err.Error(), "cannot write next to the program") {
		t.Fatalf("expected advice about the folder: %v", err)
	}
}

func TestReplaceSwapsTheProgramAndKeepsItsPermissions(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "pipechat")
	os.WriteFile(exe, aProgram("old"), 0o755)
	stage := StagePath(exe)
	os.WriteFile(stage, aProgram("new"), 0o600)

	if err := Replace(stage, exe); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(exe); !bytes.Equal(got, aProgram("new")) {
		t.Fatalf("the program was not replaced: %q", got)
	}
	if _, err := os.Stat(stage); err == nil {
		t.Fatal("the staged file should be gone")
	}
	if info, _ := os.Stat(exe); runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		t.Fatal("the new program must stay runnable")
	}
	if runtime.GOOS == "windows" {
		if got, _ := os.ReadFile(exe + ".old"); !bytes.Equal(got, aProgram("old")) {
			t.Fatal("the old program is kept aside on Windows")
		}
	}
}

func TestReplaceLeavesTheProgramAloneWhenItFails(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "pipechat")
	os.WriteFile(exe, aProgram("old"), 0o755)
	if err := Replace(filepath.Join(dir, "does-not-exist"), exe); err == nil {
		t.Fatal("replacing with nothing must fail")
	}
	if got, _ := os.ReadFile(exe); !bytes.Equal(got, aProgram("old")) {
		t.Fatal("a failed update must not damage the working program")
	}
}
