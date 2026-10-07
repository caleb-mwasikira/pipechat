package music

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

// A song that has been played is kept on this computer, so playing it again is instant, works without the
// network, and can be seeked freely. The copy is made while the song streams for the first time: no
// extra download. It lives in the user's cache folder, which the system may clear, and the oldest songs
// are removed when the folder grows past MaxCacheBytes.

// MaxCacheBytes is how much of the cache folder is kept before the least recently played songs go.
var MaxCacheBytes int64 = 512 << 20

// Cached returns the saved copy of a track, if there is a whole one. A copy that is the wrong size is ignored.
func Cached(track Track) (string, bool) {
	path, err := CachePath(track)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || (track.Size > 0 && info.Size() != track.Size) {
		return "", false
	}
	now := time.Now()
	os.Chtimes(path, now, now) // played just now: the last to be removed
	return path, true
}

// partCounter makes the names of copies in progress different, as a player may ask for a song twice at once.
var partCounter atomic.Int64

// cacheWriter collects a song as it streams to the player. If everything arrives, it is checked and
// kept; if the player stops early or the file is not the one expected, nothing is kept.
type cacheWriter struct {
	track   Track
	final   string
	part    string
	file    *os.File
	sum     hash.Hash
	written int64
	failed  bool
}

// startCache begins a copy of track, or returns nil when it cannot (no room to write, no size to check against).
func startCache(track Track) *cacheWriter {
	final, err := CachePath(track)
	if err != nil || track.Size <= 0 {
		return nil
	}
	if info, err := os.Stat(final); err == nil && info.Size() == track.Size {
		return nil // already have it
	}
	if err := os.MkdirAll(filepath.Dir(final), 0o700); err != nil {
		return nil
	}
	part := fmt.Sprintf("%s.%d-%d.part", final, os.Getpid(), partCounter.Add(1))
	file, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil
	}
	return &cacheWriter{track: track, final: final, part: part, file: file, sum: md5.New()}
}

func (c *cacheWriter) Write(data []byte) (int, error) {
	if c.failed {
		return len(data), nil // keep serving the player even if the copy cannot be kept
	}
	if _, err := c.file.Write(data); err != nil {
		c.failed = true
		return len(data), nil
	}
	c.sum.Write(data)
	c.written += int64(len(data))
	return len(data), nil
}

// finish keeps the copy if all of it arrived and it is the song it should be.
func (c *cacheWriter) finish(complete bool) {
	c.file.Close()
	good := complete && !c.failed && c.written == c.track.Size &&
		(c.track.MD5 == "" || strings.EqualFold(hex.EncodeToString(c.sum.Sum(nil)), c.track.MD5))
	if !good {
		os.Remove(c.part)
		return
	}
	if err := os.Rename(c.part, c.final); err != nil {
		os.Remove(c.part)
		return
	}
	Prune(filepath.Dir(c.final), MaxCacheBytes)
}

var _ io.Writer = (*cacheWriter)(nil)

// Prune removes the least recently played files in dir until what is left fits in max bytes. Copies still
// being made are left alone.
func Prune(dir string, max int64) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type kept struct {
		path string
		size int64
		when time.Time
	}
	var files []kept
	var total int64
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".part") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, kept{filepath.Join(dir, entry.Name()), info.Size(), info.ModTime()})
		total += info.Size()
	}
	sort.Slice(files, func(i, j int) bool { return files[i].when.Before(files[j].when) })
	for _, file := range files {
		if total <= max {
			return
		}
		if os.Remove(file.path) == nil {
			total -= file.size
		}
	}
}
