package music

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
)

// Repeat settings.
const (
	RepeatOff = "off"
	RepeatOne = "one"
	RepeatAll = "all"
)

// Shuffle puts tracks in random order. Tests replace it with something fixed.
var Shuffle = func(tracks []Track) {
	rand.Shuffle(len(tracks), func(i, j int) { tracks[i], tracks[j] = tracks[j], tracks[i] })
}

// Playlist is what plays now and what plays next: the library, the order it is played in, and the
// shuffle and repeat settings. It only decides which song is next; playing it is up to the caller.
type Playlist struct {
	Library []Track // the whole library, oldest upload first
	Queue   []Track // the order of play; empty when nothing is queued
	Pos     int     // where in the queue we are
	Shuffle bool
	Repeat  string // RepeatOff, RepeatOne or RepeatAll; empty means off
}

// Active says whether a queue is playing.
func (p *Playlist) Active() bool { return len(p.Queue) > 0 }

// Stop clears the queue.
func (p *Playlist) Stop() { p.Queue, p.Pos = nil, 0 }

// Current is the song at the queue's position.
func (p *Playlist) Current() (Track, bool) {
	if p.Pos < 0 || p.Pos >= len(p.Queue) {
		return Track{}, false
	}
	return p.Queue[p.Pos], true
}

// Start queues the whole library, beginning with the track startID (or the top when it is 0). It returns
// an error when there is no such track.
func (p *Playlist) Start(tracks []Track, startID int64) error {
	// In order means oldest upload first.
	p.Library = append([]Track(nil), tracks...)
	sort.Slice(p.Library, func(i, j int) bool { return p.Library[i].ID < p.Library[j].ID })
	first := -1
	if startID != 0 {
		if first = IndexOf(p.Library, startID); first < 0 {
			return fmt.Errorf("no track number %d, see /music", startID)
		}
	}
	p.makeQueue(first)
	return nil
}

// makeQueue orders the library for playing. first is the index in the library to begin with, or -1.
func (p *Playlist) makeQueue(first int) {
	p.Queue = append([]Track(nil), p.Library...)
	if p.Shuffle {
		var begin *Track
		if first >= 0 {
			picked := p.Library[first]
			begin = &picked
			p.Queue = append(p.Queue[:first], p.Queue[first+1:]...)
		}
		Shuffle(p.Queue)
		if begin != nil {
			p.Queue = append([]Track{*begin}, p.Queue...)
		}
	} else if first > 0 {
		p.Queue = append(append([]Track(nil), p.Queue[first:]...), p.Queue[:first]...)
	}
	p.Pos = 0
}

// Next moves on when a song has ended by itself, following the repeat setting. It returns false when the
// queue is finished.
func (p *Playlist) Next() (Track, bool) {
	switch {
	case len(p.Queue) == 0:
		return Track{}, false
	case p.Repeat == RepeatOne:
	case p.Pos+1 < len(p.Queue):
		p.Pos++
	case p.Repeat == RepeatAll:
		p.makeQueue(-1) // a new round, shuffled afresh if shuffle is on
	default:
		p.Stop()
		return Track{}, false
	}
	return p.Current()
}

// Skip goes to the following song, even when repeating one.
func (p *Playlist) Skip() (Track, bool) {
	if len(p.Queue) == 0 {
		return Track{}, false
	}
	if p.Repeat == RepeatOne {
		p.Pos = (p.Pos + 1) % len(p.Queue)
		return p.Current()
	}
	return p.Next()
}

// SetShuffle turns shuffle on or off. While a song plays, the order of what is still to come changes and
// the song itself stays first.
func (p *Playlist) SetShuffle(on bool, playing *Track) {
	p.Shuffle = on
	if len(p.Queue) > 0 && playing != nil {
		p.makeQueue(IndexOf(p.Library, playing.ID))
	}
}

// SetRepeat moves to the next repeat setting (off, all, one) when given nothing, else sets it. It returns
// false for an unknown setting.
func (p *Playlist) SetRepeat(setting string) bool {
	switch setting {
	case RepeatOff, RepeatOne, RepeatAll:
		p.Repeat = setting
	case "":
		p.Repeat = map[string]string{RepeatOff: RepeatAll, "": RepeatAll, RepeatAll: RepeatOne, RepeatOne: RepeatOff}[p.Repeat]
	default:
		return false
	}
	return true
}

// Modes describes the shuffle and repeat settings, like " [shuffle, repeat all]", or "" when both are off.
func (p *Playlist) Modes() string {
	var modes []string
	if p.Shuffle {
		modes = append(modes, "shuffle")
	}
	if p.Repeat != RepeatOff && p.Repeat != "" {
		modes = append(modes, "repeat "+p.Repeat)
	}
	if len(modes) == 0 {
		return ""
	}
	return " [" + strings.Join(modes, ", ") + "]"
}

// IndexOf finds a track by its number, or -1.
func IndexOf(tracks []Track, id int64) int {
	for index, track := range tracks {
		if track.ID == id {
			return index
		}
	}
	return -1
}
