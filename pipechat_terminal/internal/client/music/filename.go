package music

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// FileName is the name a saved song gets: "Artist - Title.ext", with anything a file system dislikes
// replaced, and never a path.
func FileName(track Track) string {
	base := track.Title
	if track.Artist != "" {
		base = track.Artist + " - " + track.Title
	}
	base = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, base)
	base = strings.Trim(strings.TrimSpace(base), ".")
	if runes := []rune(base); len(runes) > 120 {
		base = string(runes[:120])
	}
	if base == "" {
		base = "track-" + strconv.FormatInt(track.ID, 10)
	}
	ext := strings.ToLower(strings.Trim(track.Ext, "."))
	if ext == "" || strings.ContainsAny(ext, `/\.`) {
		ext = "audio"
	}
	return base + "." + ext
}

// UniquePath is dir/name, or "name (2).ext", "name (3).ext"... when that is taken: a saved song never
// overwrites a file that is already there.
func UniquePath(dir string, name string) string {
	path := filepath.Join(dir, name)
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for n := 2; ; n++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path
		}
		path = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, n, ext))
	}
}
