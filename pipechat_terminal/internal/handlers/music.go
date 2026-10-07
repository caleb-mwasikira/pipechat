package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"pipechat/internal/database"
)

// Music. People upload songs on the website, which keeps them in storage/app/private/music and
// lists them in the tracks table. Logged-in terminals list and download them here, then play
// them with a program on their own computer.

const tracksListed = 500

var audioTypes = map[string]string{
	"mp3": "audio/mpeg", "m4a": "audio/mp4", "aac": "audio/aac", "ogg": "audio/ogg",
	"oga": "audio/ogg", "opus": "audio/ogg", "wav": "audio/wav", "flac": "audio/flac",
}

// GET /api/music
func listMusic(w http.ResponseWriter, r *http.Request) {
	tracks, err := database.ListTracks(r.Context(), Pool, tracksListed)
	if err != nil {
		serverError(w, "list tracks", err)
		return
	}
	writeJSON(w, http.StatusOK, tracks)
}

// GET /api/music/{id}/file. Range requests work, so a player can seek.
func musicFile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad track")
		return
	}
	track, err := database.GetTrack(r.Context(), Pool, id)
	if err != nil {
		serverError(w, "get track", err)
		return
	}
	if track == nil {
		writeError(w, http.StatusNotFound, "no such track")
		return
	}

	contentType := audioTypes[strings.ToLower(track.Ext)]
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	serveStored(w, r, track.Path, contentType, "")
}

// DELETE /api/music/{id}: the person who uploaded the song, or an administrator, takes it out of the library.
func deleteTrack(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad track")
		return
	}
	path, removeFile, err := database.DeleteTrack(r.Context(), Pool, currentSession(r).userID, id)
	switch {
	case errors.Is(err, database.ErrNoSuchFile):
		writeError(w, http.StatusNotFound, "no such track")
	case errors.Is(err, database.ErrNotAllowed):
		writeError(w, http.StatusForbidden, "only the person who uploaded this song, or an administrator, can delete it")
	case err != nil:
		serverError(w, "delete track", err)
	default:
		if removeFile {
			removeStored(path)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
