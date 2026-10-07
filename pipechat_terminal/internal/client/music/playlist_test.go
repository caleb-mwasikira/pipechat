package music

import (
	"fmt"
	"strings"
	"testing"
)

func library() []Track {
	// Handed over newest first, as the server lists it.
	return []Track{{ID: 3, Title: "three"}, {ID: 1, Title: "one"}, {ID: 2, Title: "two"}}
}

func ids(tracks []Track) string {
	var out []string
	for _, track := range tracks {
		out = append(out, fmt.Sprint(track.ID))
	}
	return strings.Join(out, " ")
}

// play follows a playlist for n songs, calling Next each time one ends.
func play(p *Playlist, n int) string {
	var played []string
	track, ok := p.Current()
	for i := 0; i < n && ok; i++ {
		played = append(played, fmt.Sprint(track.ID))
		track, ok = p.Next()
	}
	return strings.Join(played, " ")
}

func TestPlaysOldestFirstAndFinishes(t *testing.T) {
	var p Playlist
	if err := p.Start(library(), 0); err != nil {
		t.Fatal(err)
	}
	if got := play(&p, 10); got != "1 2 3" {
		t.Fatalf("played %q", got)
	}
	if p.Active() {
		t.Fatal("the queue should be finished")
	}
}

func TestStartingFromATrackCarriesOnAndWrapsRound(t *testing.T) {
	var p Playlist
	if err := p.Start(library(), 2); err != nil {
		t.Fatal(err)
	}
	if got := ids(p.Queue); got != "2 3 1" {
		t.Fatalf("queue %q", got)
	}
	if err := p.Start(library(), 99); err == nil || !strings.Contains(err.Error(), "no track number 99") {
		t.Fatalf("an unknown track: %v", err)
	}
}

func TestShuffleKeepsTheStartingTrackFirst(t *testing.T) {
	previous := Shuffle
	Shuffle = func(tracks []Track) {
		for i, j := 0, len(tracks)-1; i < j; i, j = i+1, j-1 {
			tracks[i], tracks[j] = tracks[j], tracks[i]
		}
	}
	defer func() { Shuffle = previous }()

	p := Playlist{Shuffle: true}
	p.Start(library(), 2)
	if got := ids(p.Queue); got != "2 3 1" {
		t.Fatalf("shuffled from 2: %q", got)
	}
	p.Start(library(), 0)
	if got := ids(p.Queue); got != "3 2 1" {
		t.Fatalf("shuffled from the top: %q", got)
	}
}

func TestRealShuffleKeepsEverySongOnce(t *testing.T) {
	p := Playlist{Shuffle: true}
	big := make([]Track, 50)
	for i := range big {
		big[i] = Track{ID: int64(i + 1)}
	}
	p.Start(big, 7)
	if p.Queue[0].ID != 7 || len(p.Queue) != 50 {
		t.Fatalf("starts with %d, %d songs", p.Queue[0].ID, len(p.Queue))
	}
	seen := map[int64]bool{}
	for _, track := range p.Queue {
		if seen[track.ID] {
			t.Fatalf("track %d twice", track.ID)
		}
		seen[track.ID] = true
	}
}

func TestRepeat(t *testing.T) {
	p := Playlist{Repeat: RepeatOne}
	p.Start(library(), 2)
	if got := play(&p, 3); got != "2 2 2" {
		t.Fatalf("repeat one: %q", got)
	}
	p.Repeat = RepeatAll
	p.Start(library(), 0)
	if got := play(&p, 7); got != "1 2 3 1 2 3 1" {
		t.Fatalf("repeat all: %q", got)
	}
}

func TestSkipGoesOnEvenWhenRepeatingOne(t *testing.T) {
	p := Playlist{Repeat: RepeatOne}
	p.Start(library(), 0)
	track, _ := p.Skip()
	if track.ID != 2 {
		t.Fatalf("skip went to %d", track.ID)
	}
	p.Repeat = RepeatOff
	p.Skip() // 3
	if _, ok := p.Skip(); ok || p.Active() {
		t.Fatal("skipping past the end finishes the queue")
	}
	if _, ok := p.Skip(); ok {
		t.Fatal("nothing to skip")
	}
}

func TestShuffleCanBeToggledWhilePlaying(t *testing.T) {
	previous := Shuffle
	Shuffle = func(tracks []Track) { tracks[0], tracks[len(tracks)-1] = tracks[len(tracks)-1], tracks[0] }
	defer func() { Shuffle = previous }()

	var p Playlist
	p.Start(library(), 0) // 1 2 3
	playing, _ := p.Current()
	p.SetShuffle(true, &playing)
	if p.Queue[0].ID != 1 || ids(p.Queue) != "1 3 2" {
		t.Fatalf("after shuffle on: %q", ids(p.Queue))
	}
	p.SetShuffle(false, &playing)
	if ids(p.Queue) != "1 2 3" {
		t.Fatalf("after shuffle off: %q", ids(p.Queue))
	}
}

func TestSettingsAndTheirText(t *testing.T) {
	var p Playlist
	if p.Modes() != "" {
		t.Fatalf("nothing set: %q", p.Modes())
	}
	for _, step := range []struct{ in, want string }{{"", "all"}, {"", "one"}, {"", "off"}, {"all", "all"}} {
		if !p.SetRepeat(step.in) || p.Repeat != step.want {
			t.Fatalf("SetRepeat(%q): %q, want %q", step.in, p.Repeat, step.want)
		}
	}
	if p.SetRepeat("sometimes") {
		t.Fatal("an unknown setting must be refused")
	}
	p.Shuffle = true
	if p.Modes() != " [shuffle, repeat all]" {
		t.Fatalf("modes: %q", p.Modes())
	}
}
