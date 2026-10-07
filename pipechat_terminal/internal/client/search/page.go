package search

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Searching inside a page of text (the help guide, the music library, the file list), the way vim searches a
// file: type /pattern, then n for the next match and N for the one before. Lines may be styled (colours); the
// styles are skipped when looking and kept when marking.

// hasUpper says whether a pattern contains a capital letter.
func hasUpper(text string) bool { return strings.ToLower(text) != text }

// occurrence finds the first match at or after byte offset from in text, as byte offsets. As in vim's
// "smartcase", capitals in the pattern make the search exact; without them, capitals do not matter.
func occurrence(text string, pattern string, from int) (start int, end int, ok bool) {
	if pattern == "" || from > len(text) {
		return 0, 0, false
	}
	if hasUpper(pattern) {
		at := strings.Index(text[from:], pattern)
		if at < 0 {
			return 0, 0, false
		}
		return from + at, from + at + len(pattern), true
	}
	start, end, ok = index(text[from:], pattern)
	return from + start, from + end, ok
}

// PageMatches returns the numbers of the lines (from 0) of content that contain pattern.
func PageMatches(content string, pattern string) []int {
	var lines []int
	for number, line := range strings.Split(content, "\n") {
		if _, _, found := occurrence(ansi.Strip(line), pattern, 0); found {
			lines = append(lines, number)
		}
	}
	return lines
}

// PageHighlight returns content with every match of pattern passed through mark. The lines in current
// (a line number) are passed through strong instead, so the match being looked at stands out from the rest.
func PageHighlight(content string, pattern string, current int, mark func(string) string, strong func(string) string) string {
	if pattern == "" {
		return content
	}
	lines := strings.Split(content, "\n")
	for number, line := range lines {
		plain := ansi.Strip(line)
		if _, _, found := occurrence(plain, pattern, 0); !found {
			continue
		}
		style := mark
		if number == current {
			style = strong
		}
		var out strings.Builder
		column, from := 0, 0 // where the unmarked text resumes, in screen columns and in bytes
		for {
			start, end, found := occurrence(plain, pattern, from)
			if !found {
				break
			}
			left := ansi.StringWidth(plain[:start])
			right := left + ansi.StringWidth(plain[start:end])
			out.WriteString(ansi.Cut(line, column, left))
			out.WriteString(style(plain[start:end]))
			column, from = right, end
		}
		out.WriteString(ansi.Cut(line, column, ansi.StringWidth(plain)))
		lines[number] = out.String()
	}
	return strings.Join(lines, "\n")
}

// Step moves from the line at position `at` in matches to the next one (direction 1) or the one before (-1),
// wrapping round at the ends, as vim does. It returns the new position in matches.
func Step(matches []int, at int, direction int) int {
	if len(matches) == 0 {
		return -1
	}
	return ((at+direction)%len(matches) + len(matches)) % len(matches)
}

// FirstFrom is the position in matches of the first line at or after the line `top`, wrapping to the first match
// when there is none below. It returns -1 when there are no matches.
func FirstFrom(matches []int, top int) int {
	for position, line := range matches {
		if line >= top {
			return position
		}
	}
	if len(matches) > 0 {
		return 0
	}
	return -1
}
