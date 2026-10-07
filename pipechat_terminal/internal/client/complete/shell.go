package complete

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"pipechat/internal/client/files"
)

// Shell completes the last word of a shell command line when Tab is pressed. The first word (with no
// slash in it) completes to a program on the PATH or `cd`; any other word completes to a file or
// folder. It returns the whole line with the word completed as far as it is certain, and every name
// the word could be when there is more than one.
func Shell(line string) (completed string, options []string) {
	start := strings.LastIndexAny(line, " \t") + 1
	// A space inside a name is typed as "\ " on Linux and macOS: it does not end the word.
	for start > 1 && line[start-2] == '\\' && runtime.GOOS != "windows" {
		start = strings.LastIndexAny(line[:start-2], " \t") + 1
	}
	prefix, word := line[:start], line[start:]
	first := strings.TrimSpace(prefix) == ""

	var names []string // candidates, folders ending in a slash
	if first && !strings.ContainsAny(word, `/\`) && !strings.HasPrefix(word, "~") && !strings.HasPrefix(word, ".") {
		names = programs(word)
	} else {
		names = paths(word)
	}
	if len(names) == 0 {
		return line, nil
	}
	sort.Strings(names)
	common := names[0]
	for _, name := range names[1:] {
		for !strings.HasPrefix(name, common) {
			common = common[:len(common)-1]
		}
	}
	if len(names) == 1 {
		if strings.HasSuffix(names[0], "/") {
			return prefix + escape(names[0]), nil
		}
		return prefix + escape(names[0]) + " ", nil // a finished word: carry on typing the next
	}
	return prefix + escape(common), names
}

func programs(typed string) []string {
	seen := map[string]bool{}
	var found []string
	add := func(name string) {
		if strings.HasPrefix(name, typed) && !seen[name] {
			seen[name] = true
			found = append(found, name)
		}
	}
	add("cd")
	for _, directory := range filepath.SplitList(os.Getenv("PATH")) {
		entries, err := os.ReadDir(directory)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasPrefix(name, typed) || entry.IsDir() {
				continue
			}
			if info, err := entry.Info(); err != nil || (runtime.GOOS != "windows" && info.Mode()&0o111 == 0) {
				continue
			}
			if runtime.GOOS == "windows" {
				name = strings.TrimSuffix(strings.TrimSuffix(name, ".exe"), ".EXE")
			}
			add(name)
		}
	}
	return found
}

func paths(word string) []string {
	typed := strings.ReplaceAll(word, `\ `, " ")
	cut := strings.LastIndexAny(typed, `/\`) + 1
	typedDir, base := typed[:cut], typed[cut:]
	directory := "."
	if typedDir != "" {
		directory = files.ExpandHome(typedDir)
	}
	if typed == "~" {
		return []string{"~/"}
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil
	}
	var found []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, base) || (strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".")) {
			continue
		}
		isDir := entry.IsDir()
		if entry.Type()&os.ModeSymlink != 0 {
			if info, err := os.Stat(filepath.Join(directory, name)); err == nil {
				isDir = info.IsDir()
			}
		}
		if isDir {
			name += "/"
		}
		found = append(found, typedDir+name)
	}
	return found
}

func escape(text string) string {
	if runtime.GOOS == "windows" {
		return text
	}
	return strings.ReplaceAll(text, " ", `\ `)
}
