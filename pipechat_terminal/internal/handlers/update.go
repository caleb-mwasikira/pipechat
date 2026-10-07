package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"pipechat/internal/database"
	"pipechat/internal/version"
)

// Client updates. The admin uploads each build on the website; the terminal asks here which one is newest
// for its system and, if it is newer than the running one, downloads it. Both answers are public, as the
// same files are on the website's download page.

type versionResponse struct {
	Version string `json:"version"`
	URL     string `json:"url"` // relative to this server
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
	MD5     string `json:"md5"`
}

var (
	systems       = map[string]bool{"linux": true, "windows": true, "macos": true}
	processorKind = map[string]bool{"amd64": true, "arm64": true}
)

// GET /api/version?os=linux&arch=amd64: the newest client for a system.
func latestVersion(w http.ResponseWriter, r *http.Request) {
	osName, arch := r.URL.Query().Get("os"), r.URL.Query().Get("arch")
	if !systems[osName] || !processorKind[arch] {
		writeError(w, http.StatusBadRequest, "os must be linux, windows or macos, and arch amd64 or arm64")
		return
	}
	releases, err := database.ListReleases(r.Context(), Pool, osName, arch)
	if err != nil {
		serverError(w, "list releases", err)
		return
	}
	var newest *database.Release
	for index := range releases {
		if !version.Valid(releases[index].Version) {
			continue
		}
		if newest == nil || version.Compare(releases[index].Version, newest.Version) > 0 {
			newest = &releases[index]
		}
	}
	if newest == nil {
		writeError(w, http.StatusNotFound, "no client has been published for that system")
		return
	}
	writeJSON(w, http.StatusOK, versionResponse{
		Version: newest.Version, URL: "/api/update/" + strconv.FormatInt(newest.ID, 10),
		Size: newest.Size, SHA256: newest.SHA256, MD5: newest.MD5,
	})
}

// GET /api/update/{id}: the program file itself.
func updateFile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad download")
		return
	}
	release, err := database.GetRelease(r.Context(), Pool, id)
	if err != nil {
		serverError(w, "get release", err)
		return
	}
	if release == nil {
		writeError(w, http.StatusNotFound, "no such download")
		return
	}
	serveStored(w, r, release.Path, "application/octet-stream", release.Filename)
}
