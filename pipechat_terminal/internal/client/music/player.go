package music

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Music is played by a program already on the computer: the terminal itself has no sound code. The
// first one found from this list is used.

type player struct {
	name    string
	streams bool // can play an http:// address, so no download is needed
	args    func(source string) []string
}

var players = []player{
	{"mpv", true, func(source string) []string { return []string{"--no-video", "--really-quiet", "--no-terminal", source} }},
	{"ffplay", true, func(source string) []string { return []string{"-nodisp", "-autoexit", "-loglevel", "quiet", source} }},
	{"cvlc", true, func(source string) []string { return []string{"--play-and-exit", "--quiet", source} }},
	{"vlc", true, func(source string) []string { return []string{"-I", "dummy", "--play-and-exit", "--quiet", source} }},
	{"mplayer", true, func(source string) []string { return []string{"-really-quiet", "-vo", "null", source} }},
	{"afplay", false, func(source string) []string { return []string{source} }}, // macOS: files only
}

// lookPath finds a program; tests replace it.
var LookPath = exec.LookPath

var ErrNoPlayer = errors.New("no audio player found; install mpv (or ffplay, vlc, mplayer) and try again")

// Launcher plays something with the player found on this computer.
type Launcher struct {
	Streams bool // the source can be an http:// address; otherwise it must be a file
	Command func(source string) *exec.Cmd
}

// FindPlayer picks the first player that is installed.
func FindPlayer() (*Launcher, error) {
	for _, candidate := range players {
		if found, err := LookPath(candidate.name); err == nil {
			candidate := candidate
			return &Launcher{Streams: candidate.streams, Command: func(source string) *exec.Cmd {
				return exec.Command(found, candidate.args(source)...)
			}}, nil
		}
	}
	if runtime.GOOS == "windows" {
		// Windows has a media player built in, reachable from PowerShell. It plays files.
		if found, err := LookPath("powershell"); err == nil {
			return &Launcher{Command: func(path string) *exec.Cmd {
				script := fmt.Sprintf(`Add-Type -AssemblyName presentationCore; $p = New-Object System.Windows.Media.MediaPlayer; `+
					`$p.Open([uri]%s); $p.Play(); while (-not $p.NaturalDuration.HasTimeSpan) { Start-Sleep -Milliseconds 200 }; `+
					`Start-Sleep -Seconds ([math]::Ceiling($p.NaturalDuration.TimeSpan.TotalSeconds))`, powershellQuote(path))
				return exec.Command(found, "-NoProfile", "-NonInteractive", "-Command", script)
			}}, nil
		}
	}
	return nil, ErrNoPlayer
}

func powershellQuote(text string) string {
	return "'" + strings.ReplaceAll(text, "'", "''") + "'"
}
