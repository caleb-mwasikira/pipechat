// Package complete works out what to suggest as someone types a command: the commands themselves, and
// the file or folder names after /upload and /download. It only looks at the text and the disk; showing
// the suggestion is up to the screen.
package complete

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"pipechat/internal/client/files"
)

// Commands is every command that can be suggested, most useful first: "/" suggests /help.
var Commands = []string{
	"/help", "/create", "/create ", "/join ", "/public", "/dm ", "/whois ", "/me", "/bio ", "/bio clear",
	"/groups", "/switch ", "/close", "/closed", "/reopen ", "/code", "/invite ", "/kick ", "/leave",
	"/music", "/play", "/play all", "/save ", "/save all ", "/next", "/stop", "/shuffle on", "/shuffle off",
	"/repeat off", "/repeat one", "/repeat all", "/unlist ",
	"/upload ", "/download ", "/files", "/link ", "/share ", "/search ", "/search -here ", "/search -in ", "/clear", "/update", "/exit", "/quit",
}

// Paths completes the path being typed after /upload (the file or folder) or /download (the folder to
// save into). value is the whole line typed so far; each suggestion is a whole line too, so it shows as
// grey text after the cursor.
func Paths(value string) []string {
	command, dirsOnly := "", false
	switch {
	case strings.HasPrefix(value, "/upload "):
		command = "/upload "
	case strings.HasPrefix(value, "/download "):
		command, dirsOnly = "/download ", true
	case strings.HasPrefix(value, "/save "):
		command, dirsOnly = "/save ", true
	default:
		return nil
	}
	argument := value[len(command):]
	start, quote := lastToken(argument)

	// Which argument is this? /upload PATH [-name SHORT]: the first one that is not a flag's value.
	// /download SHORT|LINK [DIR]: the second one.
	before := files.SplitArguments(argument[:start])
	positional, valueNext := 0, false
	for _, word := range before {
		switch {
		case valueNext:
			valueNext = false
		case word == "-name" || word == "--name":
			valueNext = true
		default:
			positional++
		}
	}
	if valueNext || (command == "/upload " && positional > 0) || ((command == "/download " || command == "/save ") && positional != 1) {
		return nil
	}

	raw := strings.TrimLeft(argument[start:], `"'`)
	if raw == "~" {
		return []string{value + "/"}
	}
	separator := "/"
	if filepath.Separator == '\\' && strings.Contains(raw, `\`) {
		separator = `\`
	}
	cut := strings.LastIndex(raw, separator) + 1
	typedDir, typedBase := raw[:cut], raw[cut:]
	directory := "."
	if typedDir != "" {
		directory = files.ExpandHome(typedDir)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil
	}

	type candidate struct {
		name string
		dir  bool
	}
	var found []candidate
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, typedBase) || (strings.HasPrefix(name, ".") && !strings.HasPrefix(typedBase, ".")) {
			continue
		}
		isDir := entry.IsDir()
		if entry.Type()&os.ModeSymlink != 0 {
			if info, err := os.Stat(filepath.Join(directory, name)); err == nil {
				isDir = info.IsDir()
			}
		}
		if dirsOnly && !isDir {
			continue
		}
		found = append(found, candidate{name, isDir})
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].dir != found[j].dir {
			return found[i].dir // folders first
		}
		return found[i].name < found[j].name
	})

	var suggestions []string
	for _, item := range found {
		if len(suggestions) >= 100 {
			break
		}
		completed := typedDir + item.name
		if item.dir {
			completed += separator
		}
		text := completed
		if quote != 0 || strings.ContainsAny(item.name, " \t'\"") {
			opening := string(quote)
			if quote == 0 {
				opening = `"`
			}
			text = opening + completed
			if !item.dir {
				text += opening // a folder's quote stays open, so typing can go on inside it
			}
		}
		suggestion := command + argument[:start] + text
		if strings.HasPrefix(strings.ToLower(suggestion), strings.ToLower(value)) {
			suggestions = append(suggestions, suggestion)
		}
	}
	return suggestions
}

// lastToken finds where the word being typed starts in a command's arguments, and the quote it
// opened with, if it is still open.
func lastToken(text string) (start int, quote rune) {
	for index, character := range text {
		switch {
		case quote != 0 && character == quote:
			quote = 0
		case quote != 0:
		case character == '"' || character == '\'':
			quote = character
		case character == ' ' || character == '\t':
			start = index + 1
		}
	}
	return start, quote
}
