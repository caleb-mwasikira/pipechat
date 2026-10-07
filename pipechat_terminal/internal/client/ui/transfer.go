package ui

import (
	"fmt"
	"pipechat/internal/client/files"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

var (
	barFull  = lipgloss.NewStyle().Foreground(lipgloss.Color("49"))
	barEmpty = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
)

// TransferLine draws the progress of an upload or download on one line that fits the width:
//
//	⇡ uploading photos.zip  [███████░░░░░░░]  52%  6.1 MB / 12.0 MB  2.4 MB/s  ETA 3s
func TransferLine(p files.Progress, width int, now time.Time) string {
	arrow := "⇡"
	if p.Verb == "downloading" {
		arrow = "⇣"
	}
	head := fmt.Sprintf("%s %s %s", arrow, p.Verb, Truncate(p.Name, 24))

	var tail string
	known := p.Total > 0
	if known {
		done := min(p.Done, p.Total)
		tail = fmt.Sprintf("%3d%%  %s / %s", done*100/p.Total, files.FormatSize(done), files.FormatSize(p.Total))
	} else {
		tail = files.FormatSize(p.Done)
	}
	if p.Rate >= 1 {
		tail += "  " + files.FormatSize(int64(p.Rate)) + "/s"
		if known && p.Done < p.Total {
			tail += "  ETA " + ShortDuration(time.Duration(float64(p.Total-p.Done)/p.Rate*float64(time.Second)))
		}
	}

	// The bar takes what the words leave.
	barWidth := width - lipgloss.Width(head) - lipgloss.Width(tail) - 6
	if barWidth < 8 {
		return Truncate(head+"  "+tail, width) // a narrow terminal: words only
	}
	var bar string
	if known {
		filled := int(min(p.Done, p.Total) * int64(barWidth) / p.Total)
		bar = barFull.Render(strings.Repeat("█", filled)) + barEmpty.Render(strings.Repeat("░", barWidth-filled))
	} else {
		// Not known how long it is: a block sliding to and fro, so it is plain that something is going on.
		span := barWidth - 4
		position := int(now.UnixMilli()/90) % (2 * span)
		if position > span {
			position = 2*span - position
		}
		bar = barEmpty.Render(strings.Repeat("░", position)) + barFull.Render("████") + barEmpty.Render(strings.Repeat("░", span-position))
	}
	spinner := spinnerFrames[int(now.UnixMilli()/100)%len(spinnerFrames)]
	return barFull.Render(spinner) + " " + head[len(arrow)+1:] + "  [" + bar + "]  " + tail
}

// ShortDuration writes a wait as "3s", "2m 05s" or "1h 04m".
func ShortDuration(d time.Duration) string {
	seconds := int(d.Round(time.Second).Seconds())
	switch {
	case seconds < 60:
		return fmt.Sprintf("%ds", max(seconds, 1))
	case seconds < 3600:
		return fmt.Sprintf("%dm %02ds", seconds/60, seconds%60)
	default:
		return fmt.Sprintf("%dh %02dm", seconds/3600, seconds%3600/60)
	}
}
