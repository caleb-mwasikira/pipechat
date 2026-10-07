package search

import (
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	for _, c := range []struct {
		in    string
		want  Request
		fails bool
	}{
		{"deploy friday", Request{Scope: AllChats, Query: "deploy friday"}, false},
		{"  spaced   out  ", Request{Scope: AllChats, Query: "spaced   out"}, false},
		{"-here lunch plans", Request{Scope: CurrentChat, Query: "lunch plans"}, false},
		{"-in crew budget meeting", Request{Scope: NamedChat, Chat: "crew", Query: "budget meeting"}, false},
		{`-in "late night" snacks`, Request{Scope: NamedChat, Chat: "late night", Query: "snacks"}, false},
		{"-in @bob hello", Request{Scope: NamedChat, Chat: "@bob", Query: "hello"}, false},
		{"-hereditary", Request{Scope: AllChats, Query: "-hereditary"}, false}, // only the exact flag is a flag
		{"", Request{}, true},
		{"-here", Request{}, true},
		{"-in", Request{}, true},
		{"-in crew", Request{}, true},
	} {
		got, err := Parse(c.in)
		if c.fails {
			if err == nil || !strings.Contains(err.Error(), "usage: /search") {
				t.Errorf("Parse(%q) should explain how to use it, got %v", c.in, err)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("Parse(%q) = %+v, %v; want %+v", c.in, got, err, c.want)
		}
	}
}

func TestFind(t *testing.T) {
	at := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	lines := []Line{
		{Chat: "crew", When: at, Handle: "alice", Text: "Deploy goes out on Friday"},
		{Chat: "crew", When: at, Handle: "bob", Text: "friday? I thought thursday"},
		{Chat: "@carol", When: at, Handle: "carol", Text: "no match here"},
		{Chat: "notes", When: at, Text: "a notice about FRIDAY"},
	}
	hits, total := Find(lines, "friday", 10)
	if total != 3 || len(hits) != 3 {
		t.Fatalf("found %d of %d", len(hits), total)
	}
	if hits[0].Text[hits[0].Start:hits[0].End] != "Friday" || hits[2].Text[hits[2].Start:hits[2].End] != "FRIDAY" {
		t.Fatalf("matches are marked where they are: %+v", hits)
	}
	if hits, total := Find(lines, "friday", 2); len(hits) != 2 || total != 3 {
		t.Fatalf("a limit cuts the list but not the count: %d of %d", len(hits), total)
	}
	if hits, _ := Find(lines, "", 10); len(hits) != 0 {
		t.Fatal("an empty search finds nothing")
	}
	if hits, _ := Find(lines, "zzz", 10); len(hits) != 0 {
		t.Fatal("no match, no hits")
	}
	// Characters whose lower case is a different length must not break the marked place.
	odd := []Line{{Text: "İstanbul trip İ"}}
	if hits, _ := Find(odd, "trip", 5); len(hits) != 1 || odd[0].Text[hits[0].Start:hits[0].End] != "trip" {
		t.Fatalf("offsets with unusual capitals: %+v", hits)
	}
}

func TestChatMatches(t *testing.T) {
	for _, c := range []struct {
		name, asked string
		want        bool
	}{
		{"Crew", "crew", true}, {"Crew", "cr", true}, {"crew", "crewmates", false},
		{"@bob", "bob", true}, {"@bob", "@bo", true}, {"crew", "bob", false},
	} {
		if got := ChatMatches(c.name, c.asked); got != c.want {
			t.Errorf("ChatMatches(%q, %q) = %v", c.name, c.asked, got)
		}
	}
}
