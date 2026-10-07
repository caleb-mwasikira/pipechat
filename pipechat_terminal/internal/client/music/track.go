// Package music is the music side of the terminal client: the songs of the shared library, a playlist that
// knows about shuffle and repeat, a local stream address so any player can stream a song, and finding a
// player program on this computer. It knows nothing about the screen or the chat server's protocol.
package music

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Track is one song in the shared music library.
type Track struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Ext      string `json:"ext"`
	Size     int64  `json:"size"`
	Uploader string `json:"uploader"`
}

// CachePath is where a downloaded track is kept, so playing it again is instant.
func CachePath(track Track) (string, error) {
	directory, err := os.UserCacheDir()
	if err != nil {
		directory = os.TempDir()
	}
	ext := strings.ToLower(track.Ext)
	if ext == "" || strings.ContainsAny(ext, `/\.`) {
		return "", errors.New("bad track")
	}
	return filepath.Join(directory, "pipechat", "music", strconv.FormatInt(track.ID, 10)+"."+ext), nil
}
