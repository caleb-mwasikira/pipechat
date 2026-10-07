package handlers

import "net/http"

type infoResponse struct {
	Website string `json:"website"` // where people make an account; empty when it is the same address as this server
}

// GET /api/info: facts a client wants before anyone has logged in.
func serverInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, infoResponse{Website: Config.WebsiteURL})
}
