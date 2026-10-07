package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"

	"pipechat/internal/database"
)

// Encrypted files. The client encrypts before uploading, so the server only
// stores and serves opaque bytes under a random 128-bit id:
//
//	POST /api/files        upload (login required)
//	GET  /api/files/{id}   download
//
// Whoever has the id can fetch the encrypted bytes, but they are useless without
// the key, which only travels inside end-to-end encrypted chat messages.

var fileIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

func uploadFile(w http.ResponseWriter, r *http.Request) {
	temporary, err := os.CreateTemp(Config.FilesDir, "upload-*.tmp")
	if err != nil {
		serverError(w, "create upload file", err)
		return
	}
	defer os.Remove(temporary.Name())
	defer temporary.Close()

	size, err := temporary.ReadFrom(http.MaxBytesReader(w, r.Body, Config.MaxUploadBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "file too large")
		} else {
			writeError(w, http.StatusBadRequest, "upload interrupted")
		}
		return
	}
	id := randomHex(16)
	if err := temporary.Close(); err != nil || os.Rename(temporary.Name(), filepath.Join(Config.FilesDir, id)) != nil {
		serverError(w, "store upload", err)
		return
	}
	if err := database.AddFile(r.Context(), Pool, id, currentSession(r).handle, size); err != nil {
		os.Remove(filepath.Join(Config.FilesDir, id))
		serverError(w, "record upload", err)
		return
	}
	writeJSON(w, http.StatusCreated, fileResponse{ID: id, Size: size})
}

func downloadFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	file, err := os.Open(filepath.Join(Config.FilesDir, id))
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found; it may have expired")
		return
	}
	defer file.Close()
	info, _ := file.Stat()
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, "", info.ModTime(), file) // also answers Range requests
}

// CleanFiles deletes files older than Config.FileDays, every hour. Run it in a goroutine.
func CleanFiles() {
	for {
		cutoff := time.Now().Add(-time.Duration(Config.FileDays) * 24 * time.Hour)
		ids, err := database.ExpiredFiles(context.Background(), Pool, cutoff)
		if err != nil {
			log.Printf("expired files: %v", err)
		}
		for _, id := range ids {
			if err := os.Remove(filepath.Join(Config.FilesDir, id)); err == nil || errors.Is(err, os.ErrNotExist) {
				database.DeleteFile(context.Background(), Pool, id)
				log.Printf("deleted expired file %s", id)
			}
		}
		time.Sleep(time.Hour)
	}
}

// DELETE /api/files/{id}: the person who uploaded a file, or the owner of the website group it was shared in,
// deletes it. Everyone else gets a refusal.
func deleteFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !fileIDPattern.MatchString(id) {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	own := currentSession(r)
	switch err := database.DeleteSharedFile(r.Context(), Pool, own.userID, own.handle, id); {
	case errors.Is(err, database.ErrNoSuchFile):
		writeError(w, http.StatusNotFound, "file not found; it may have expired")
	case errors.Is(err, database.ErrNotAllowed):
		writeError(w, http.StatusForbidden, err.Error())
	case err != nil:
		serverError(w, "delete file", err)
	default:
		os.Remove(filepath.Join(Config.FilesDir, id))
		w.WriteHeader(http.StatusNoContent)
	}
}
