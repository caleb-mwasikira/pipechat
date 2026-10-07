// Package lookup finds one thing in a list from what a person typed: its number, its name, or the start of
// its MD5 hash. The same rules serve the shared files and the music library, so "/download 9e10",
// "/download holiday.zip" and "/play #3" all work the way people expect.
package lookup

import (
	"fmt"
	"strconv"
	"strings"
)

// Item is something that can be found.
type Item struct {
	Number int64    // shown in the list; "#3" and "3" find it
	Hash   string   // MD5, in hex; any start of it, from MinHash characters, finds it
	Names  []string // what it is called: the file name, the title, "artist - title"...
}

// MinHash is how many characters of a hash must be typed. Fewer would match too many things by chance, and
// would be mistaken for numbers.
const MinHash = 4

// Find returns the index of the item the text means. The rules, in this order:
//
//  1. "#3" is item number 3;
//  2. a name that is exactly the text (capitals do not matter);
//  3. all digits, and an item with that number;
//  4. hex digits, MinHash or more, that start exactly one item's hash;
//  5. text that is part of exactly one item's name.
//
// When the text fits several items it says which, so the person can be more exact.
func Find(items []Item, text string) (int, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return -1, fmt.Errorf("say which one: its number, its name, or the start of its hash")
	}
	lower := strings.ToLower(text)

	if rest, ok := strings.CutPrefix(text, "#"); ok {
		number, err := strconv.ParseInt(rest, 10, 64)
		if err == nil {
			for index, item := range items {
				if item.Number == number {
					return index, nil
				}
			}
		}
		return -1, fmt.Errorf("there is no number %s in the list", rest)
	}

	if matches := matching(items, func(item Item) bool {
		for _, name := range item.Names {
			if strings.EqualFold(name, text) {
				return true
			}
		}
		return false
	}); len(matches) == 1 {
		return matches[0], nil
	} else if len(matches) > 1 {
		return -1, ambiguous(items, matches, text)
	}

	if number, err := strconv.ParseInt(text, 10, 64); err == nil {
		for index, item := range items {
			if item.Number == number {
				return index, nil
			}
		}
	}

	if len(lower) >= MinHash && isHex(lower) {
		matches := matching(items, func(item Item) bool { return item.Hash != "" && strings.HasPrefix(strings.ToLower(item.Hash), lower) })
		if len(matches) == 1 {
			return matches[0], nil
		} else if len(matches) > 1 {
			return -1, ambiguous(items, matches, text)
		}
	}

	matches := matching(items, func(item Item) bool {
		for _, name := range item.Names {
			if strings.Contains(strings.ToLower(name), lower) {
				return true
			}
		}
		return false
	})
	switch len(matches) {
	case 0:
		return -1, fmt.Errorf("nothing matches %q", text)
	case 1:
		return matches[0], nil
	}
	return -1, ambiguous(items, matches, text)
}

func matching(items []Item, test func(Item) bool) []int {
	var found []int
	for index, item := range items {
		if test(item) {
			found = append(found, index)
		}
	}
	return found
}

func ambiguous(items []Item, matches []int, text string) error {
	var shown []string
	for _, index := range matches {
		if len(shown) == 5 {
			shown = append(shown, fmt.Sprintf("and %d more", len(matches)-5))
			break
		}
		item := items[index]
		name := ""
		if len(item.Names) > 0 {
			name = item.Names[0]
		}
		shown = append(shown, fmt.Sprintf("#%d %s %s", item.Number, Short(item.Hash), name))
	}
	return fmt.Errorf("%q fits more than one: %s. Use more of the hash, or the number", text, strings.Join(shown, ", "))
}

func isHex(text string) bool {
	for _, character := range text {
		if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f') {
			return false
		}
	}
	return text != ""
}

// Short is the start of a hash shown in lists: enough to type, short enough to read.
func Short(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	if hash == "" {
		return "--------"
	}
	return hash
}
