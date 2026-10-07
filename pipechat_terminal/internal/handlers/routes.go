package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Routes is the list of every URL of the chat server and the handler that answers it.
//
//	POST   /api/auth/login              website email + password -> token, vault (auth.go)
//	GET    /api/files/{id}              download an encrypted file            (files.go)
//
// Everything below needs "Authorization: Bearer <token>":
//
//	PUT    /api/identity                first keys, or fresh ones              (auth.go)
//	GET    /api/sync                    website groups to create/share/receive (sync.go)
//	POST   /api/sync/groups             owner: the chat id of a website group  (sync.go)
//	POST   /api/sync/escrow             let the server hold a group key        (sync.go)
//	POST   /api/sync/kick               creator removes a member               (sync.go)
//	POST   /api/sync/grants             send a group key sealed to a member    (sync.go)
//	DELETE /api/sync/grants/{group}     key received                           (sync.go)
//	PUT    /api/vault                   replace your sealed vault             (auth.go)
//	GET    /api/users/{handle}          a user's public key, bio and status    (auth.go)
//	PUT    /api/profile                 change your bio                        (auth.go)
//	POST   /api/direct/{handle}         tell a user you opened a direct chat  (auth.go)
//	DELETE /api/direct/{handle}         forget such a notice                  (auth.go)
//	POST   /api/groups                  create a group                        (groups.go)
//	POST   /api/groups/{gid}/join       join with the join token, get history (groups.go)
//	GET    /api/groups/{gid}/online     who has this chat open now             (groups.go)
//	DELETE /api/groups/{gid}/membership leave                                 (groups.go)
//	POST   /api/groups/{gid}/messages   send ciphertext                       (groups.go)
//	POST   /api/open                    make your group public by name         (open.go)
//	GET    /api/open[/{name}]           list public groups, or get one to join (open.go)
//	DELETE /api/open/{name}             make it private again                  (open.go)
//	GET    /api/music                   the music library                      (music.go)
//	GET    /api/music/{id}/file         download a track (supports ranges)     (music.go)
//	POST   /api/invitations             email a group's invite code to someone (invitations.go)
//	GET    /api/ws                      WebSocket: incoming messages, notices (websocket.go)
//	POST   /api/files                   upload an encrypted file              (files.go)
func Routes() http.Handler {
	router := chi.NewRouter()

	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	router.Get("/up", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("OK"))
	})

	router.Route("/api", func(api chi.Router) {
		api.Post("/auth/login", login)
		api.Get("/files/{id}", downloadFile)

		api.Group(func(api chi.Router) {
			api.Use(requireLogin)

			api.Put("/vault", saveVault)
			api.Put("/identity", saveIdentity)
			api.Put("/profile", saveBio)
			api.Get("/sync", getSync)
			api.Post("/sync/groups", syncGroupCreated)
			api.Post("/sync/escrow", syncEscrow)
			api.Post("/sync/kick", syncKick)
			api.Post("/sync/shares", syncShare)
			api.Post("/sync/grants", syncGrant)
			api.Delete("/sync/grants/{group}", syncGrantReceived)
			api.Get("/users/{handle}", lookupUser)
			api.Post("/direct/{handle}", notifyDirect)
			api.Delete("/direct/{handle}", clearDirect)

			api.Post("/groups", createGroup)
			api.Post("/groups/{gid}/join", joinGroup)
			api.Delete("/groups/{gid}/membership", leaveGroup)
			api.Get("/groups/{gid}/online", groupOnline)
			api.Post("/groups/{gid}/messages", sendMessage)

			api.Post("/open", listOpenGroup)
			api.Get("/open", listOpenGroups)
			api.Get("/open/{name}", getOpenGroup)
			api.Delete("/open/{name}", unlistOpenGroup)
			api.Get("/music", listMusic)
			api.Get("/music/{id}/file", musicFile)
			api.Post("/invitations", inviteByEmail)
			api.Get("/ws", chatSocket)
			api.Post("/files", uploadFile)
		})
	})

	return router
}
