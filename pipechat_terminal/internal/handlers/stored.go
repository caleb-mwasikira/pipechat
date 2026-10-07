package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// serveStored sends a file kept in the website's storage folder (music, client downloads). The path comes
// from the database, but it is never trusted to stay inside the storage folder. Range requests work, so a
// player can seek and a download can be resumed. downloadName, when given, is the file name offered.
func serveStored(w http.ResponseWriter, r *http.Request, relative string, contentType string, downloadName string) {
	root, err := filepath.Abs(Config.StorageDir)
	if err != nil {
		serverError(w, "storage folder", err)
		return
	}
	full := filepath.Join(root, filepath.Clean("/"+relative))
	if !strings.HasPrefix(full, root+string(filepath.Separator)) {
		writeError(w, http.StatusNotFound, "no such file")
		return
	}
	file, err := os.Open(full)
	if err != nil {
		writeError(w, http.StatusNotFound, "the file is missing")
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "the file is missing")
		return
	}
	w.Header().Set("Content-Type", contentType)
	if downloadName != "" {
		w.Header().Set("Content-Disposition", `attachment; filename="`+downloadName+`"`)
	}
	http.ServeContent(w, r, "", info.ModTime(), file)
}

// removeStored deletes a file from the website's storage folder, with the same care about staying inside it.
func removeStored(relative string) {
	root, err := filepath.Abs(Config.StorageDir)
	if err != nil {
		return
	}
	full := filepath.Join(root, filepath.Clean("/"+relative))
	if strings.HasPrefix(full, root+string(filepath.Separator)) {
		os.Remove(full)
	}
}
