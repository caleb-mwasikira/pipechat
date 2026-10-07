package files

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

// What is on disk, described for a preview before anything is sent.

const (
	previewLines = 18   // lines of a text file shown
	previewWalk  = 5000 // entries looked at in a folder before giving up counting
	previewSniff = 4096
)

// Describe names what a file is and, for text, returns its first lines.
func Describe(path string) (kind string, preview []string) {
	file, err := os.Open(path)
	if err != nil {
		return "unreadable: " + err.Error(), nil
	}
	defer file.Close()
	head := make([]byte, previewSniff)
	count, _ := io.ReadFull(file, head)
	head = head[:count]

	extension := strings.ToLower(filepath.Ext(path))
	byName := mime.TypeByExtension(extension)
	sniffed := http.DetectContentType(head)

	switch {
	case strings.HasPrefix(sniffed, "image/"):
		file.Seek(0, io.SeekStart)
		if config, format, err := image.DecodeConfig(file); err == nil {
			return fmt.Sprintf("%s image, %d × %d pixels", strings.ToUpper(format), config.Width, config.Height), nil
		}
		return "image (" + sniffed + ")", nil
	case strings.HasPrefix(sniffed, "audio/") || strings.HasPrefix(byName, "audio/"):
		return "audio (" + firstNonEmpty(byName, sniffed) + ")", nil
	case strings.HasPrefix(sniffed, "video/") || strings.HasPrefix(byName, "video/"):
		return "video (" + firstNonEmpty(byName, sniffed) + ")", nil
	case sniffed == "application/pdf":
		return "PDF document", nil
	case sniffed == "application/zip", sniffed == "application/x-gzip":
		return "compressed archive (" + sniffed + ")", nil
	case isText(head):
		file.Seek(0, io.SeekStart)
		return "text" + textHint(extension), firstLines(file, previewLines)
	}
	return "binary file" + hintFor(byName), nil
}

func textHint(extension string) string {
	if extension == "" {
		return ""
	}
	return " (" + strings.TrimPrefix(extension, ".") + ")"
}

func hintFor(byName string) string {
	if byName == "" {
		return ""
	}
	return " (" + byName + ")"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// isText says whether the first bytes of a file look like text rather than binary data.
func isText(head []byte) bool {
	if len(head) == 0 {
		return true
	}
	// A multi-byte character may be cut off at the end of the sample.
	for cut := 0; cut < 4 && cut < len(head); cut++ {
		if utf8.Valid(head[:len(head)-cut]) {
			for _, b := range head {
				if b == 0 || (b < 0x20 && b != '\n' && b != '\r' && b != '\t' && b != 0x1b && b != '\f') {
					return false
				}
			}
			return true
		}
	}
	return false
}

func firstLines(reader io.Reader, count int) []string {
	data, _ := io.ReadAll(io.LimitReader(reader, 64<<10))
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	more := len(lines) > count
	if more {
		lines = lines[:count]
	}
	for index, line := range lines {
		lines[index] = strings.Map(func(r rune) rune {
			if r == '\t' {
				return ' '
			}
			if r < 0x20 || r == 0x7f {
				return -1
			}
			return r
		}, line)
	}
	if more {
		lines = append(lines, "…")
	}
	return lines
}

// Entry is one item in a folder.
type Entry struct {
	Path string // relative to the folder, with a / at the end for a folder
	Size int64
	Dir  bool
}

// FolderInfo is what a folder holds. When it is huge the count stops at a limit and Truncated is set.
type FolderInfo struct {
	Files, Folders int
	Total          int64
	Truncated      bool
	Entries        []Entry // sorted by path
}

// Summarize counts what is inside a folder.
func Summarize(root string) FolderInfo {
	var info FolderInfo
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}
		if info.Files+info.Folders >= previewWalk {
			info.Truncated = true
			return filepath.SkipAll
		}
		relative, _ := filepath.Rel(root, path)
		stat, statErr := d.Info()
		if statErr != nil {
			return nil
		}
		if d.IsDir() {
			info.Folders++
			info.Entries = append(info.Entries, Entry{Path: relative + "/", Dir: true})
		} else {
			info.Files++
			info.Total += stat.Size()
			info.Entries = append(info.Entries, Entry{Path: relative, Size: stat.Size()})
		}
		return nil
	})
	sort.Slice(info.Entries, func(i, j int) bool { return info.Entries[i].Path < info.Entries[j].Path })
	return info
}
