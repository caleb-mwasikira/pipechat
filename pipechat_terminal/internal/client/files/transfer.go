package files

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"time"

	"pipechat/internal/crypto"
)

// Sharing files and folders. An upload is a tar.gz of the path, encrypted with a
// fresh random key, streamed to the server. The share link is
//
//	http(s)://server:port/api/files/<id>#<key>
//
// The key is in the fragment, which is never sent to the server, and the link
// itself only travels inside end-to-end encrypted chat messages.

// Env is what an upload or download needs from the program around it: the server's address, the login
// token, and a way to tell the screen how far it has got.
type Env interface {
	URL() string
	Token() string
	Emit(message any)
}

// Progress is reported while a file goes up or comes down, a few times a second.
type Progress struct {
	Verb  string  // "uploading" or "downloading"
	Name  string  // the file's short name
	Done  int64   // bytes so far
	Total int64   // bytes in all, or 0 when it is not known ahead
	Rate  float64 // bytes a second, smoothed
}

// Done is the result of an upload.
type Done struct {
	Name  string
	Hash  string // MD5 of the file's bytes, or of the packed folder
	Link  string
	Bytes int64
	Files int
	Err   error
}

// Downloaded is the result of a download.
type Downloaded struct {
	Name    string
	Hash    string // MD5 of what arrived
	Checked bool   // it was compared with the MD5 that came with the share, and matched
	Dir     string
	Entries []string
	Bytes   int64
	Err     error
}

// Upload packs, encrypts and uploads a file or folder, then returns the share link.
func Upload(env Env, source string, name string) Done {
	result := Done{Name: name}
	key, keyText, err := crypto.NewFileKey()
	if err != nil {
		result.Err = err
		return result
	}

	var packed atomic.Int64
	reader, writer := io.Pipe()
	go func() {
		var err error
		result.Files, result.Hash, err = WriteArchive(writer, source, key, &packed)
		writer.CloseWithError(err)
	}()

	stop := make(chan struct{})
	defer close(stop)
	go reportProgress(env, "uploading", name, &packed, SourceSize(source), stop)

	request, _ := http.NewRequest("POST", env.URL()+"/api/files", reader)
	request.Header.Set("Content-Type", "application/octet-stream")
	request.Header.Set("Authorization", "Bearer "+env.Token())
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		reader.CloseWithError(err)
		result.Err = err
		return result
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		var failure struct{ Error string }
		json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&failure)
		result.Err = fmt.Errorf("server said: %s (%s)", failure.Error, response.Status)
		return result
	}
	var stored struct{ ID string }
	if err := json.NewDecoder(response.Body).Decode(&stored); err != nil || stored.ID == "" {
		result.Err = errors.New("unexpected answer from the server")
		return result
	}
	result.Bytes = packed.Load()
	result.Link = env.URL() + "/api/files/" + stored.ID + "#" + keyText
	return result
}

// reportProgress tells the screen how far a transfer has got, until stop is closed. Total is the size to
// reach, or 0 when it is not known.
func reportProgress(env Env, verb string, name string, counter *atomic.Int64, total int64, stop chan struct{}) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	var rate float64
	last, lastDone := time.Now(), int64(0)
	env.Emit(Progress{Verb: verb, Name: name, Total: total})
	for {
		select {
		case <-ticker.C:
			done, now := counter.Load(), time.Now()
			if elapsed := now.Sub(last).Seconds(); elapsed > 0 {
				instant := float64(done-lastDone) / elapsed
				if rate == 0 {
					rate = instant
				} else {
					rate = 0.7*rate + 0.3*instant // smooth out the jumps
				}
				last, lastDone = now, done
			}
			env.Emit(Progress{Verb: verb, Name: name, Done: done, Total: total, Rate: rate})
		case <-stop:
			return
		}
	}
}

// SourceSize adds up the bytes of the files an upload will read.
func SourceSize(source string) int64 {
	var total int64
	filepath.WalkDir(source, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		if info, err := entry.Info(); err == nil && info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// Download fetches, decrypts and unpacks a shared file or folder into
// targetDir. Nothing is written there unless the whole download verifies, and
// nothing existing is overwritten.
func Download(env Env, link string, targetDir string, name string, want string) Downloaded {
	result := Downloaded{Name: name, Dir: targetDir}
	address, key, err := ParseLink(link)
	if err != nil {
		result.Err = err
		return result
	}
	response, err := http.Get(address)
	if err != nil {
		result.Err = err
		return result
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		result.Err = errors.New("the file is gone from the server (files expire)")
		return result
	}
	if response.StatusCode != http.StatusOK {
		result.Err = fmt.Errorf("server answered %s", response.Status)
		return result
	}

	var received atomic.Int64
	stop := make(chan struct{})
	defer close(stop)
	go reportProgress(env, "downloading", name, &received, max(response.ContentLength, 0), stop)

	counted := &countingReader{reader: response.Body, total: &received}
	result.Entries, result.Bytes, result.Hash, result.Err = ExtractArchive(counted, key, targetDir, want)
	result.Checked = result.Err == nil && want != ""
	return result
}
