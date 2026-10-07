package lookup

import (
	"strings"
	"testing"
)

var items = []Item{
	{Number: 1, Hash: "9e107d9d372bb6826bd81d3542a419d6", Names: []string{"holiday.zip"}},
	{Number: 2, Hash: "e4d909c290d0fb1ca068ffaddf22cbd0", Names: []string{"notes.txt", "Notes"}},
	{Number: 12, Hash: "9e10ffff000000000000000000000000", Names: []string{"holiday-2.zip"}},
	{Number: 30, Hash: "", Names: []string{"old-share"}}, // from before hashes existed
}

func TestFind(t *testing.T) {
	for _, c := range []struct {
		text string
		want int
	}{
		{"holiday.zip", 0},      // a name, exactly
		{"HOLIDAY.ZIP", 0},      // capitals do not matter
		{"#2", 1},               // an explicit number
		{"2", 1},                // a plain number
		{"12", 2},               // numbers are not mistaken for hashes
		{"e4d9", 1},             // the start of a hash
		{"E4D909C290D0FB1C", 1}, // in capitals too
		{"9e107d", 0},           // enough of a shared start to tell them apart
		{"notes", 1},            // a name that is part of one name
		{"old", 3},              // part of a name, for something with no hash
		{"Notes", 1},            // one of several names
	} {
		got, err := Find(items, c.text)
		if err != nil || got != c.want {
			t.Errorf("Find(%q) = %d, %v; want %d", c.text, got, err, c.want)
		}
	}
}

func TestFindSaysWhenItCannotTell(t *testing.T) {
	for _, c := range []struct{ text, want string }{
		{"9e10", "fits more than one"},    // two hashes start like this
		{"holiday", "fits more than one"}, // two names contain this
		{"", "say which one"},
		{"#99", "no number 99"},
		{"#x", "no number x"},
		{"zzzz", "nothing matches"},
		{"e4", "nothing matches"}, // too short to be a hash, and it is in no name
	} {
		_, err := Find(items, c.text)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("Find(%q): %v, want an error with %q", c.text, err, c.want)
		}
	}
	_, err := Find(items, "9e10")
	for _, show := range []string{"#1 9e107d9d holiday.zip", "#12 9e10ffff holiday-2.zip", "more of the hash"} {
		if !strings.Contains(err.Error(), show) {
			t.Errorf("the message should list the candidates: %v", err)
		}
	}
}

func TestAnExactNameBeatsEverythingElse(t *testing.T) {
	odd := []Item{{Number: 1, Hash: "abcdef0123456789", Names: []string{"abcd"}}, {Number: 2, Hash: "abcd0000", Names: []string{"other"}}}
	if got, err := Find(odd, "abcd"); err != nil || got != 0 {
		t.Fatalf("a file really called abcd: %d %v", got, err)
	}
	if got, err := Find([]Item{{Number: 7, Names: []string{"7"}}, {Number: 8, Names: []string{"x"}}}, "8"); err != nil || got != 1 {
		t.Fatalf("number: %d %v", got, err)
	}
}

func TestShort(t *testing.T) {
	if Short("9e107d9d372bb6826bd81d3542a419d6") != "9e107d9d" || Short("") != "--------" || Short("abc") != "abc" {
		t.Fatal("short hashes")
	}
}
