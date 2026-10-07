package ui

import (
	"pipechat/internal/client/files"
	"strings"
	"testing"
	"time"
)

func TestTransferLineShowsPercentRateAndEta(t *testing.T) {
	now := time.Now()
	line := TransferLine(files.Progress{Verb: "uploading", Name: "photos.zip", Done: 6 << 20, Total: 12 << 20, Rate: 2 << 20}, 100, now)
	for _, want := range []string{"uploading", "photos.zip", " 50%", "6.0 MB / 12.0 MB", "2.0 MB/s", "ETA 3s", "█", "░"} {
		if !strings.Contains(line, want) {
			t.Errorf("progress line lacks %q: %s", want, line)
		}
	}
	if width := len([]rune(stripStyles(line))); width > 100 {
		t.Errorf("the line is %d wide, wanted at most 100", width)
	}

	finished := TransferLine(files.Progress{Verb: "downloading", Name: "x", Done: 10, Total: 10}, 80, now)
	if !strings.Contains(finished, "100%") || strings.Contains(finished, "ETA") || !strings.Contains(finished, "downloading") {
		t.Errorf("a finished transfer: %s", finished)
	}

	// Unknown length: no percentage, but still a moving bar, and still the bytes so far.
	unknown := TransferLine(files.Progress{Verb: "downloading", Name: "x", Done: 3 << 20}, 80, now)
	if strings.Contains(unknown, "%") || !strings.Contains(unknown, "3.0 MB") || !strings.Contains(unknown, "████") {
		t.Errorf("unknown length: %s", unknown)
	}
	if a, b := TransferLine(files.Progress{Verb: "downloading", Name: "x", Done: 1}, 80, now), TransferLine(files.Progress{Verb: "downloading", Name: "x", Done: 1}, 80, now.Add(600*time.Millisecond)); a == b {
		t.Error("the unknown-length bar should move")
	}

	// A narrow terminal gets words only, and never overflows.
	narrow := TransferLine(files.Progress{Verb: "uploading", Name: "a-very-long-file-name.tar.gz", Done: 5, Total: 10, Rate: 1000}, 40, now)
	if width := len([]rune(stripStyles(narrow))); width > 40 {
		t.Errorf("narrow line is %d wide: %s", width, narrow)
	}
}

// stripStyles removes colour codes, so widths can be counted.
func stripStyles(text string) string {
	var out strings.Builder
	skipping := false
	for _, character := range text {
		switch {
		case character == 0x1b:
			skipping = true
		case skipping && character == 'm':
			skipping = false
		case !skipping:
			out.WriteRune(character)
		}
	}
	return out.String()
}

func TestShortDuration(t *testing.T) {
	for in, want := range map[time.Duration]string{400 * time.Millisecond: "1s", 3 * time.Second: "3s", 125 * time.Second: "2m 05s", 3900 * time.Second: "1h 05m"} {
		if got := ShortDuration(in); got != want {
			t.Errorf("%v: got %q want %q", in, got, want)
		}
	}
}
