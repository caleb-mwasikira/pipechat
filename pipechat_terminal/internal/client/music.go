package client

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"os"
	"os/exec"
	"path/filepath"
	"pipechat/internal/client/ui"
	"strings"
	"time"

	"pipechat/internal/client/files"
	"pipechat/internal/client/music"
)

// Music, from the terminal. The library is on the website; here we play it: /play makes a queue of the
// whole library (shuffled or in order), and each track is streamed to a player on this computer, or
// downloaded first for a player that cannot stream. A song that ends starts the next one. The playlist
// logic is in the music package; this file is the commands and the screen around it.

// Results reported back to the screen.
type musicList struct {
	tracks []music.Track
	err    error
}
type musicStart struct {
	tracks []music.Track
	query  string // which song to start with: a number, a hash or a name; empty to begin at the top
	err    error
}
type musicReady struct {
	track  music.Track
	source string // an http:// address to stream, or a file
	err    error
}
type musicEnded struct {
	cmd *exec.Cmd
	err error
}

// nowPlaying is the song coming out of the speakers.
type nowPlaying struct {
	track   music.Track
	cmd     *exec.Cmd
	started time.Time
}

// showMusic opens the library as a page, like the help guide: aligned columns, the song that is playing marked,
// and the commands that go with it. Esc closes it.
func (m *Model) showMusic(result musicList) tea.Cmd {
	current := m.groups[m.current]
	if result.err != nil {
		m.notice(current, "could not get the music library: "+result.err.Error())
		return nil
	}
	tracks := result.tracks
	var playing int64
	if m.playing != nil {
		playing = m.playing.track.ID
	}
	command := m.openPage("music", func(width int) string { return ui.MusicLibrary(width, tracks, playing, m.list.Modes()) })
	m.pageFooter = "↑/↓ scroll  ·  / search  ·  Esc back  ·  select text to copy  ·  then /play" // after openPage, which clears it
	return command
}

// playCommand handles /play and /play SONG, where SONG is a number, the start of a hash, or a name.
func (m *Model) playCommand(current *groupState, argument string) {
	query := strings.TrimSpace(argument)
	if query == "all" {
		query = ""
	}
	if m.loadingMusic {
		m.notice(current, "still loading the last track")
		return
	}
	if _, err := music.FindPlayer(); err != nil {
		m.notice(current, err.Error())
		return
	}
	m.loadingMusic = true
	server := m.server
	go func() {
		tracks, err := server.Music()
		server.report(musicStart{tracks: tracks, query: query, err: err})
	}()
}

// beginQueue builds the queue from the library and starts playing it.
func (m *Model) beginQueue(result musicStart) {
	m.loadingMusic = false
	current := m.groups[m.current]
	if result.err != nil {
		m.notice(current, "could not get the music library: "+result.err.Error())
		return
	}
	if len(result.tracks) == 0 {
		m.notice(current, "the library is empty; upload songs on the website, under Music")
		return
	}
	var startID int64
	if result.query != "" {
		track, err := music.Find(result.tracks, result.query)
		if err != nil {
			m.notice(current, err.Error()+"; see /music")
			return
		}
		startID = track.ID
	}
	m.stopMusic()
	if err := m.list.Start(result.tracks, startID); err != nil {
		m.notice(current, err.Error())
		return
	}
	m.playAt()
}

// playAt starts the song the playlist is at: stream it, or download it for a player that cannot.
func (m *Model) playAt() {
	track, ok := m.list.Current()
	if !ok {
		return
	}
	launcher, err := music.FindPlayer()
	if err != nil {
		m.notice(m.groups[m.current], err.Error())
		m.list.Stop()
		return
	}
	m.loadingMusic = true
	server := m.server

	// A song played before is on this computer: play that, with no network and free seeking.
	if path, ok := music.Cached(track); ok {
		go server.report(musicReady{track: track, source: path})
		return
	}

	var proxy *music.Proxy
	if launcher.Streams {
		if m.proxy == nil || !m.proxy.For(server) {
			if m.proxy != nil {
				m.proxy.Close()
			}
			m.proxy, err = music.StartProxy(server)
			if err != nil {
				m.loadingMusic = false
				m.notice(m.groups[m.current], "could not start streaming: "+err.Error())
				m.list.Stop()
				return
			}
		}
		proxy = m.proxy
	}
	go func() {
		if proxy != nil {
			server.report(musicReady{track: track, source: proxy.URL(track)})
			return
		}
		path, err := music.CachePath(track)
		if err == nil {
			if info, statErr := os.Stat(path); statErr != nil || info.Size() != track.Size {
				err = server.DownloadTrack(track.ID, path, nil)
			}
		}
		server.report(musicReady{track: track, source: path, err: err})
	}()
}

// startTrack plays a track that is ready, replacing whatever was playing.
func (m *Model) startTrack(result musicReady) {
	m.loadingMusic = false
	current := m.groups[m.current]
	if !m.list.Active() {
		return // stopped while it was loading
	}
	if result.err != nil {
		m.notice(current, "could not play "+result.track.Title+": "+result.err.Error())
		m.list.Stop()
		return
	}
	launcher, err := music.FindPlayer()
	if err != nil {
		m.notice(current, err.Error())
		m.list.Stop()
		return
	}
	command := launcher.Command(result.source)
	if err := command.Start(); err != nil {
		m.notice(current, "could not start the player: "+err.Error())
		m.list.Stop()
		return
	}
	m.playing = &nowPlaying{track: result.track, cmd: command, started: time.Now()}
	m.notice(current, "♪ playing "+result.track.Title+m.list.Modes()+"  (/next, /stop)")
	server := m.server
	go func() { server.report(musicEnded{cmd: command, err: command.Wait()}) }()
}

// songEnded runs when the player exits by itself: play the next song, or stop.
func (m *Model) songEnded(result musicEnded) {
	if m.playing == nil || m.playing.cmd != result.cmd {
		return // we stopped it ourselves
	}
	song, took := m.playing.track, time.Since(m.playing.started)
	m.playing = nil
	if result.err != nil && took < 3*time.Second {
		// A player that fails at once would otherwise be restarted for ever.
		m.notice(m.groups[m.current], "the player stopped with an error on "+song.Title+": "+result.err.Error())
		m.list.Stop()
		return
	}
	m.afterSong(m.list.Next)
}

// afterSong moves the playlist on with next (Next or Skip) and plays what it gives, or ends the queue.
func (m *Model) afterSong(next func() (music.Track, bool)) {
	if !m.list.Active() {
		return
	}
	if _, ok := next(); !ok {
		m.notice(m.groups[m.current], "the queue is finished")
		return
	}
	m.playAt()
}

// skipCommand handles /next.
func (m *Model) skipCommand(current *groupState) {
	if !m.list.Active() {
		m.notice(current, "nothing is playing")
		return
	}
	m.killPlayer()
	m.afterSong(m.list.Skip)
}

// repeatCommand handles /repeat off|one|all.
func (m *Model) repeatCommand(current *groupState, argument string) {
	if !m.list.SetRepeat(argument) {
		m.notice(current, "usage: /repeat off|one|all")
		return
	}
	m.notice(current, "repeat: "+m.list.Repeat)
}

// shuffleCommand handles /shuffle on|off.
func (m *Model) shuffleCommand(current *groupState, argument string) {
	on := !m.list.Shuffle
	switch argument {
	case "on":
		on = true
	case "off":
		on = false
	case "":
	default:
		m.notice(current, "usage: /shuffle on|off")
		return
	}
	var song *music.Track
	if m.playing != nil {
		song = &m.playing.track
	}
	m.list.SetShuffle(on, song)
	if on {
		m.notice(current, "shuffle: on")
	} else {
		m.notice(current, "shuffle: off")
	}
}

// killPlayer ends the song that is playing, without ending the queue.
func (m *Model) killPlayer() {
	if m.playing == nil {
		return
	}
	playing := m.playing
	m.playing = nil // so its exit is not taken for the song ending
	if playing.cmd.Process != nil {
		playing.cmd.Process.Kill()
	}
}

// stopMusic ends the song and the queue.
func (m *Model) stopMusic() {
	m.killPlayer()
	m.list.Stop()
	m.loadingMusic = false
}

// reportShare tells the server about a file shared in a chat, for the website group's page.
func (m *Model) reportShare(g *groupState, name string, size int64, hash string) {
	if g == nil || g.secret == nil || g.direct {
		return
	}
	id, server := g.secret.GIDHex(), m.server
	m.server.enqueue(func() { server.ReportShare(id, name, size, hash) })
}

// musicSaved is the result of /save.
type musicSaved struct {
	dir   string
	saved []string // the file names written
	err   error
}

// saveCommand handles /save NUMBER|all [DIR]: keep songs from the library as ordinary files.
func (m *Model) saveCommand(current *groupState, argument string) {
	arguments := files.SplitArguments(argument)
	if len(arguments) == 0 || len(arguments) > 2 {
		m.notice(current, "usage: /save SONG [FOLDER]  or  /save all [FOLDER]  (SONG is a number, a hash or a name; see /music)")
		return
	}
	query := arguments[0]
	if m.busy {
		m.notice(current, "another transfer is still running")
		return
	}
	dir := defaultMusicDir()
	if len(arguments) == 2 {
		dir = files.ExpandHome(arguments[1])
	}

	m.busy, m.transfer = true, "saving music"
	m.notice(current, "saving to "+dir+"...")
	server := m.server
	go func() {
		result := musicSaved{dir: dir}
		tracks, err := server.Music()
		if err == nil {
			var chosen []music.Track
			if query == "all" {
				chosen = tracks
			} else if track, findErr := music.Find(tracks, query); findErr != nil {
				err = fmt.Errorf("%w; see /music", findErr)
			} else {
				chosen = []music.Track{track}
			}
			if err == nil && len(chosen) == 0 {
				err = fmt.Errorf("the library is empty")
			}
			if err == nil {
				err = os.MkdirAll(dir, 0o755)
			}
			for index := 0; err == nil && index < len(chosen); index++ {
				track := chosen[index]
				name := track.Title
				if len(chosen) > 1 {
					name = fmt.Sprintf("%d/%d %s", index+1, len(chosen), track.Title)
				}
				path := music.UniquePath(dir, music.FileName(track))
				err = server.DownloadTrack(track.ID, path, func(done, total int64) {
					server.emit(files.Progress{Verb: "downloading", Name: name, Done: done, Total: total})
				})
				if err == nil {
					result.saved = append(result.saved, filepath.Base(path))
				}
			}
		}
		result.err = err
		server.report(result)
	}()
}

// defaultMusicDir is where songs are saved when no folder is named: the Music folder if there is one.
func defaultMusicDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		for _, name := range []string{"Music", "Downloads"} {
			if info, err := os.Stat(filepath.Join(home, name)); err == nil && info.IsDir() {
				return filepath.Join(home, name)
			}
		}
	}
	return "."
}

// handleMusicSaved reports how /save went.
func (m *Model) handleMusicSaved(result musicSaved) {
	m.busy, m.transfer, m.progress = false, "", nil
	m.layout()
	current := m.groups[m.current]
	switch {
	case result.err != nil && len(result.saved) == 0:
		m.notice(current, "could not save: "+result.err.Error())
		return
	case result.err != nil:
		m.notice(current, "stopped early: "+result.err.Error())
	}
	where := result.dir
	if abs, err := filepath.Abs(result.dir); err == nil {
		where = abs
	}
	m.notice(current, fmt.Sprintf("saved %d song(s) to %s:", len(result.saved), where))
	for _, name := range result.saved {
		m.notice(current, "  "+name)
	}
}
