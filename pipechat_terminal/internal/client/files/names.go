// Package files is the file-sharing side of the terminal client: packing and encrypting a file or folder,
// uploading it, downloading and unpacking a share link, and describing what is on disk before it is sent.
// It knows nothing about the screen; it reports progress through the Env it is given.
package files

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"pipechat/internal/crypto"
)

var (
	// ShortNamePattern is what a file's short name may look like.
	ShortNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,40}$`)
	linkPathPattern  = regexp.MustCompile(`^/api/files/[0-9a-f]{32}$`)
)

// ShortName turns a file name into a valid short name.
func ShortName(name string) string {
	name = regexp.MustCompile(`[^A-Za-z0-9._-]`).ReplaceAllString(name, "_")
	if len(name) > 40 {
		name = name[:40]
	}
	return name
}

// SplitArguments splits a command line on spaces, keeping "quoted parts" together.
func SplitArguments(text string) []string {
	var arguments []string
	var current strings.Builder
	var quote rune
	started := false
	for _, character := range text {
		switch {
		case quote != 0 && character == quote:
			quote = 0
		case quote != 0:
			current.WriteRune(character)
		case character == '"' || character == '\'':
			quote, started = character, true
		case character == ' ' || character == '\t':
			if started || current.Len() > 0 {
				arguments = append(arguments, current.String())
				current.Reset()
				started = false
			}
		default:
			current.WriteRune(character)
		}
	}
	if started || current.Len() > 0 {
		arguments = append(arguments, current.String())
	}
	return arguments
}

func ExpandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

func FormatSize(bytes int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	size := float64(bytes)
	unit := 0
	for size >= 1024 && unit < len(units)-1 {
		size /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d B", bytes)
	}
	return fmt.Sprintf("%.1f %s", size, units[unit])
}

// ParseLink checks a share link and returns the address to fetch and the key.
func ParseLink(link string) (address string, key []byte, err error) {
	parsed, err := url.Parse(strings.TrimSpace(link))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || !linkPathPattern.MatchString(parsed.Path) {
		return "", nil, errors.New("not a valid file link")
	}
	key, err = crypto.ParseFileKey(parsed.Fragment)
	if err != nil {
		return "", nil, err
	}
	parsed.Fragment, parsed.RawQuery = "", ""
	return parsed.String(), key, nil
}
