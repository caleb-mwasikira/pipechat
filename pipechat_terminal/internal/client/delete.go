package client

import (
	"fmt"
	"strings"

	"pipechat/internal/client/files"
	"pipechat/internal/client/music"
)

// /delete FILE and /delete-music SONG. The server decides who may: the person who uploaded it, or (for a file)
// the owner of the website group it was shared in, or (for a song) an administrator.

type fileDeleted struct {
	name string
	err  error
}

type trackDeleted struct {
	title string
	err   error
}

func (m *Model) deleteCommand(current *groupState, argument string) {
	argument = strings.TrimSpace(argument)
	if argument == "" {
		m.notice(current, "usage: /delete NAME|NUMBER|HASH  deletes a file you uploaded, or one shared in a group you own (see /files)")
		return
	}
	file, err := m.findShared(argument)
	if err != nil {
		m.notice(current, err.Error())
		return
	}
	id := files.FileID(file.Link)
	if id == "" {
		m.notice(current, file.Name+" is not a file held by this server, so it cannot be deleted here")
		return
	}
	server, name := m.server, file.Name
	go func() { server.report(fileDeleted{name: name, err: server.DeleteFile(id)}) }()
	m.notice(current, "deleting "+name+"...")
}

func (m *Model) fileDeleted(result fileDeleted) {
	current := m.groups[m.current]
	switch {
	case result.err == nil:
		delete(m.vault.Files, result.name)
		m.saveVault()
		m.notice(current, "deleted "+result.name+". Nobody can download it any more.")
	case isNotFound(result.err):
		// Already gone (expired, or deleted by someone else): forget it here too.
		delete(m.vault.Files, result.name)
		m.saveVault()
		m.notice(current, result.name+" was already gone from the server; removed it from your list")
	default:
		m.notice(current, "could not delete "+result.name+": "+result.err.Error())
	}
}

func (m *Model) deleteMusicCommand(current *groupState, argument string) {
	argument = strings.TrimSpace(argument)
	if argument == "" {
		m.notice(current, "usage: /delete-music SONG  (a number, a hash or a name; see /music). Only the uploader or an administrator can delete a song.")
		return
	}
	server := m.server
	go func() {
		tracks, err := server.Music()
		var track music.Track
		if err == nil {
			track, err = music.Find(tracks, argument)
			if err != nil {
				err = fmt.Errorf("%w; see /music", err)
			}
		}
		if err != nil {
			server.report(trackDeleted{title: argument, err: err})
			return
		}
		server.report(trackDeleted{title: track.Title, err: server.DeleteTrack(track.ID)})
	}()
	m.notice(current, "deleting "+argument+"...")
}

func (m *Model) trackDeleted(result trackDeleted) {
	current := m.groups[m.current]
	if result.err != nil {
		m.notice(current, "could not delete "+result.title+": "+result.err.Error())
		return
	}
	m.notice(current, "deleted the song "+result.title+" from the library")
}
