// Package search looks for text in what was said in chats. It works on plain lines handed to it, so it knows
// nothing about the screen, the history file or encryption.
package search

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Line is one thing said (or noted) in a chat.
type Line struct {
	Chat   string // the chat's name
	When   time.Time
	Handle string // who said it; empty for a notice from the program
	Text   string
}

// Hit is a line that contains the search text, with where in the text it does.
type Hit struct {
	Line
	Start, End int // byte offsets of the first match inside Text
}

// Scope says which chats to look in.
type Scope int

const (
	AllChats Scope = iota
	CurrentChat
	NamedChat
)

// Request is what a person asked for.
type Request struct {
	Scope Scope
	Chat  string // for NamedChat
	Query string
}

// Parse reads the arguments of /search:
//
//	/search TEXT              every chat
//	/search -here TEXT        only the chat you are in
//	/search -in NAME TEXT     only the chat called NAME (quote the name if it has spaces)
//
// TEXT is everything that is left, so it may contain spaces and does not need quotes.
func Parse(argument string) (Request, error) {
	argument = strings.TrimSpace(argument)
	request := Request{Scope: AllChats}
	switch {
	case argument == "-here" || strings.HasPrefix(argument, "-here "):
		request.Scope = CurrentChat
		argument = strings.TrimSpace(strings.TrimPrefix(argument, "-here"))
	case argument == "-in" || strings.HasPrefix(argument, "-in "):
		rest := strings.TrimSpace(strings.TrimPrefix(argument, "-in"))
		name, remainder := firstWord(rest)
		if name == "" {
			return request, errors.New("usage: /search -in CHAT TEXT")
		}
		request.Scope, request.Chat = NamedChat, name
		argument = strings.TrimSpace(remainder)
	}
	request.Query = argument
	if request.Query == "" {
		return request, errors.New("usage: /search TEXT   or   /search -here TEXT   or   /search -in CHAT TEXT")
	}
	return request, nil
}

// firstWord splits off the first word, which may be in quotes.
func firstWord(text string) (word string, rest string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", ""
	}
	if text[0] == '"' || text[0] == '\'' {
		if end := strings.IndexByte(text[1:], text[0]); end >= 0 {
			return text[1 : 1+end], text[2+end:]
		}
	}
	word, rest, _ = strings.Cut(text, " ")
	return word, rest
}

// Find returns the lines that contain query, ignoring capitals, oldest first, at most limit of them. total
// is how many matched, which may be more than were returned.
func Find(lines []Line, query string, limit int) (hits []Hit, total int) {
	if query == "" {
		return nil, 0
	}
	for _, line := range lines {
		start, end, ok := index(line.Text, query)
		if !ok {
			continue
		}
		total++
		if len(hits) < limit {
			hits = append(hits, Hit{Line: line, Start: start, End: end})
		}
	}
	return hits, total
}

// index finds the first place text holds needle, ignoring capitals, and returns it as byte offsets into text.
// It compares rune by rune, so characters whose lower case has a different length (İ) cannot shift the answer.
func index(text string, needle string) (start int, end int, ok bool) {
	for i := range text {
		// A match is at most a few bytes longer than the needle even with unusual capitals.
		for j := i + 1; j <= len(text) && j-i <= 4*len(needle); j++ {
			if !utf8.RuneStart(textByte(text, j)) && j < len(text) {
				continue
			}
			if strings.EqualFold(text[i:j], needle) {
				return i, j, true
			}
		}
	}
	return 0, 0, false
}

// textByte is the byte at j, or a rune start when j is the end of the text.
func textByte(text string, j int) byte {
	if j >= len(text) {
		return 'a'
	}
	return text[j]
}

// ChatMatches says whether a chat's name is the one asked for: the whole name, or the start of it, ignoring capitals.
func ChatMatches(name string, asked string) bool {
	name, asked = strings.ToLower(strings.TrimPrefix(name, "@")), strings.ToLower(strings.TrimPrefix(asked, "@"))
	return name == asked || strings.HasPrefix(name, asked)
}
