// Package update keeps the terminal client up to date: it asks the chat server for the newest client for this
// system, downloads it, checks that what arrived is the program it should be, swaps it in for the running
// one, and starts it again. It knows nothing about the screen.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"pipechat/internal/version"
)

// Current is the version of this program. It is set when the program is built:
//
//	go build -ldflags "-X pipechat/internal/client/update.Current=1.2.3"
//
// A program built without it is "dev", which never offers an update.
var Current = "dev"

// Release is a client the server can hand out.
type Release struct {
	Version string `json:"version"`
	URL     string `json:"url"` // relative to the server
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
	MD5     string `json:"md5"`
}

// Platform names this computer the way the server does: linux, windows or macos, and amd64 or arm64.
// ok is false for anything else, which has no downloads.
func Platform() (osName string, arch string, ok bool) {
	switch runtime.GOOS {
	case "linux", "windows":
		osName = runtime.GOOS
	case "darwin":
		osName = "macos"
	default:
		return "", "", false
	}
	switch runtime.GOARCH {
	case "amd64", "arm64":
		return osName, runtime.GOARCH, true
	}
	return "", "", false
}

// Newer says whether the release is newer than the program that is running. A "dev" build is never
// offered an update, so working on the code is not interrupted.
func (r Release) Newer() bool {
	return Current != "dev" && version.Valid(Current) && version.Valid(r.Version) && version.Compare(r.Version, Current) > 0
}

// Check asks the server for the newest client for this computer. It returns nil when there is none.
func Check(ctx context.Context, server string) (*Release, error) {
	osName, arch, ok := Platform()
	if !ok {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	address := server + "/api/version?os=" + url.QueryEscape(osName) + "&arch=" + url.QueryEscape(arch)
	request, err := http.NewRequestWithContext(ctx, "GET", address, nil)
	if err != nil {
		return nil, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	switch {
	case response.StatusCode == http.StatusNotFound:
		return nil, nil // nothing published for this system
	case response.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("the server answered %s", response.Status)
	}
	var release Release
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&release); err != nil {
		return nil, errors.New("the server's answer about updates was not understood")
	}
	if !version.Valid(release.Version) || !strings.HasPrefix(release.URL, "/api/update/") {
		return nil, errors.New("the server's answer about updates was not understood")
	}
	return &release, nil
}

// Executable is the path of the running program, with links followed, so that is the file that gets replaced.
func Executable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// StagePath is where a new version is put before it replaces the running one: next to it, so the swap
// is one rename on the same disk, and so we know early whether we are allowed to write there.
func StagePath(exe string) string { return exe + ".new" }

// Download fetches the release into stage and checks it. progress is called with the bytes so far and the
// total, if the server said it. Nothing that fails the checks stays on disk.
func Download(ctx context.Context, server string, release Release, stage string, progress func(done, total int64)) error {
	request, err := http.NewRequestWithContext(ctx, "GET", server+release.URL, nil)
	if err != nil {
		return err
	}
	response, err := (&http.Client{}).Do(request) // a program can take longer than a short timeout allows
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("the server answered %s", response.Status)
	}

	file, err := os.OpenFile(stage, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("cannot write next to the program (%s): %w", filepath.Dir(stage), err)
	}
	hash := sha256.New()
	counter := &progressWriter{progress: progress, total: max(response.ContentLength, release.Size)}
	written, copyErr := io.Copy(io.MultiWriter(file, hash, counter), response.Body)
	closeErr := file.Close()
	if err := firstError(copyErr, closeErr); err != nil {
		os.Remove(stage)
		return err
	}
	switch {
	case release.Size > 0 && written != release.Size:
		os.Remove(stage)
		return fmt.Errorf("the download is incomplete (%d of %d bytes)", written, release.Size)
	case release.SHA256 != "" && !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), release.SHA256):
		os.Remove(stage)
		return errors.New("the download does not match its checksum, so it was thrown away")
	}
	if err := checkProgramFormat(stage); err != nil {
		os.Remove(stage)
		return err
	}
	return nil
}

type progressWriter struct {
	progress func(done, total int64)
	total    int64
	done     int64
}

func (p *progressWriter) Write(data []byte) (int, error) {
	p.done += int64(len(data))
	if p.progress != nil {
		p.progress(p.done, p.total)
	}
	return len(data), nil
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// checkProgramFormat makes sure the file is a program for this system, not, say, an error page that was
// saved under the right name.
func checkProgramFormat(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	head := make([]byte, 4)
	if _, err := io.ReadFull(file, head); err != nil {
		return errors.New("the download is not a program")
	}
	magic := string(head)
	ok := false
	switch runtime.GOOS {
	case "linux":
		ok = magic == "\x7fELF"
	case "windows":
		ok = magic[:2] == "MZ"
	case "darwin":
		// Mach-O, in either byte order, 32 or 64 bit, or a universal binary.
		for _, known := range []string{"\xfe\xed\xfa\xce", "\xfe\xed\xfa\xcf", "\xce\xfa\xed\xfe", "\xcf\xfa\xed\xfe", "\xca\xfe\xba\xbe"} {
			ok = ok || magic == known
		}
	default:
		ok = true
	}
	if !ok {
		return errors.New("the download is not a program for this system")
	}
	return nil
}

// Replace puts the program in stage in place of the one at exe. On Windows a running program cannot be
// overwritten but it can be renamed, so the old one is moved aside (and removed by Cleanup next time).
func Replace(stage string, exe string) error {
	if info, err := os.Stat(exe); err == nil {
		os.Chmod(stage, info.Mode().Perm()|0o111)
	}
	if runtime.GOOS != "windows" {
		if err := os.Rename(stage, exe); err != nil {
			return fmt.Errorf("cannot replace %s: %w", exe, err)
		}
		return nil
	}
	old := exe + ".old"
	os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("cannot move %s aside: %w", exe, err)
	}
	if err := os.Rename(stage, exe); err != nil {
		os.Rename(old, exe) // put the working program back
		return fmt.Errorf("cannot replace %s: %w", exe, err)
	}
	return nil
}

// Cleanup removes what an earlier update left behind. Call it when the program starts.
func Cleanup() {
	if exe, err := Executable(); err == nil {
		os.Remove(exe + ".old")
		os.Remove(StagePath(exe))
	}
}
