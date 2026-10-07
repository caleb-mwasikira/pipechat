package handlers

import (
	"net/http"
	"os"
	"path/filepath"
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

	// The path comes from the database, but never trust it to stay inside the storage folder.
	root, err := filepath.Abs(Config.StorageDir)
	if err != nil {
		serverError(w, "music folder", err)
		return
	}
	full := filepath.Join(root, filepath.Clean("/"+track.Path))
	if !strings.HasPrefix(full, root+string(filepath.Separator)) {
		writeError(w, http.StatusNotFound, "no such track")
		return
	}
	file, err := os.Open(full)
	if err != nil {
		writeError(w, http.StatusNotFound, "the file for this track is missing")
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "the file for this track is missing")
		return
	}
	contentType := audioTypes[strings.ToLower(track.Ext)]
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	http.ServeContent(w, r, "", info.ModTime(), file)
}
