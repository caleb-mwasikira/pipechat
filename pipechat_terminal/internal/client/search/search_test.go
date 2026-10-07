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

func TestPageMatchesSkipStylesAndUseSmartCase(t *testing.T) {
	page := "\x1b[1mHELP\x1b[0m guide\n  /play SONG   play the \x1b[36mlibrary\x1b[0m\n  /Save NUMBER  keep a song\nnothing here"
	if got := PageMatches(page, "song"); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("lower case ignores capitals: %v", got)
	}
	if got := PageMatches(page, "SONG"); len(got) != 1 || got[0] != 1 {
		t.Fatalf("a capital makes it exact: %v", got)
	}
	if got := PageMatches(page, "library"); len(got) != 1 || got[0] != 1 {
		t.Fatalf("text inside colour codes is found: %v", got)
	}
	if got := PageMatches(page, "\x1b[36m"); len(got) != 0 {
		t.Fatalf("the colour codes themselves are not text: %v", got)
	}
	if len(PageMatches(page, "")) != 0 || len(PageMatches(page, "zzz")) != 0 {
		t.Fatal("no pattern and no match find nothing")
	}
}

func TestPageHighlightMarksEveryMatchAndKeepsTheStyles(t *testing.T) {
	mark := func(s string) string { return "[" + s + "]" }
	strong := func(s string) string { return "{" + s + "}" }
	page := "song one, SONG two\n\x1b[36mthe song\x1b[0m\nplain"

	got := PageHighlight(page, "song", 0, mark, strong)
	lines := strings.Split(got, "\n")
	if lines[0] != "{song} one, {SONG} two" {
		t.Fatalf("the line looked at uses the strong mark, on every match: %q", lines[0])
	}
	if !strings.Contains(lines[1], "[song]") || !strings.Contains(lines[1], "\x1b[36m") || ansiPlain(lines[1]) != "the [song]" {
		t.Fatalf("other lines get the plain mark and keep their colour: %q", lines[1])
	}
	if lines[2] != "plain" {
		t.Fatalf("lines with no match are left as they are: %q", lines[2])
	}
	if PageHighlight(page, "", 0, mark, strong) != page {
		t.Fatal("no pattern, no change")
	}
	// Wide characters count as two columns, so the mark lands on the right text.
	wide := PageHighlight("日本語 song 日本語", "song", -1, mark, strong)
	if wide != "日本語 [song] 日本語" {
		t.Fatalf("wide characters: %q", wide)
	}
}

func ansiPlain(s string) string {
	var out strings.Builder
	skipping := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			skipping = true
		case skipping && r == 'm':
			skipping = false
		case !skipping:
			out.WriteRune(r)
		}
	}
	return out.String()
}

func TestStepAndFirstFromWrapRoundLikeVim(t *testing.T) {
	matches := []int{3, 10, 40}
	for _, c := range []struct{ at, dir, want int }{{0, 1, 1}, {2, 1, 0}, {0, -1, 2}, {1, -1, 0}} {
		if got := Step(matches, c.at, c.dir); got != c.want {
			t.Errorf("Step(%d, %d) = %d, want %d", c.at, c.dir, got, c.want)
		}
	}
	if Step(nil, 0, 1) != -1 {
		t.Error("no matches, no step")
	}
	for top, want := range map[int]int{0: 0, 3: 0, 4: 1, 11: 2, 41: 0} {
		if got := FirstFrom(matches, top); got != want {
			t.Errorf("FirstFrom(%d) = %d, want %d", top, got, want)
		}
	}
	if FirstFrom(nil, 5) != -1 {
		t.Error("nothing to start from")
	}
}
