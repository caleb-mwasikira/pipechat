package music

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileName(t *testing.T) {
	for _, c := range []struct {
		track Track
		want  string
	}{
		{Track{ID: 1, Title: "Blue Hour", Artist: "The Tests", Ext: "mp3"}, "The Tests - Blue Hour.mp3"},
		{Track{ID: 2, Title: "No Artist", Ext: "FLAC"}, "No Artist.flac"},
		{Track{ID: 3, Title: "../../etc/passwd", Artist: "x", Ext: "mp3"}, "x - .._.._etc_passwd.mp3"},
		{Track{ID: 4, Title: `a:b*c?"d"<e>|f`, Ext: "ogg"}, "a_b_c__d__e__f.ogg"},
		{Track{ID: 5, Title: "  ...  ", Ext: "wav"}, "track-5.wav"},
		{Track{ID: 6, Title: "x", Ext: "../exe"}, "x.audio"},
		{Track{ID: 7, Title: "Tab\there\nnewline", Ext: "mp3"}, "Tab_here_newline.mp3"},
	} {
		got := FileName(c.track)
		if got != c.want {
			t.Errorf("%+v: got %q want %q", c.track, got, c.want)
		}
		if strings.ContainsAny(got, `/\`) {
			t.Errorf("%q must not be a path", got)
		}
	}
	long := FileName(Track{Title: strings.Repeat("é", 300), Ext: "mp3"})
	if len([]rune(long)) > 125 {
		t.Errorf("a long title is cut: %d characters", len([]rune(long)))
	}
}

func TestUniquePathNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	first := UniquePath(dir, "song.mp3")
	if first != filepath.Join(dir, "song.mp3") {
		t.Fatalf("first: %q", first)
	}
	os.WriteFile(first, []byte("x"), 0o644)
	second := UniquePath(dir, "song.mp3")
	if second != filepath.Join(dir, "song (2).mp3") {
		t.Fatalf("second: %q", second)
	}
	os.WriteFile(second, []byte("x"), 0o644)
	if third := UniquePath(dir, "song.mp3"); third != filepath.Join(dir, "song (3).mp3") {
		t.Fatalf("third: %q", third)
	}
}
