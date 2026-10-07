package handlers

// The JSON bodies of the API. Keys, passwords and plain text never appear here:
// "auth" is a key derived from the password, "vault" and "ct" are sealed blobs.

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token  string   `json:"token"`
	Handle string   `json:"handle"`
	Vault  []byte   `json:"vault,omitempty"`  // empty on the very first login
	PubKey string   `json:"pubkey,omitempty"` // empty on the very first login
	Peers  []string `json:"peers,omitempty"`  // handles that asked for a direct chat
}

type identityRequest struct {
	PubKey string `json:"pubkey"` // X25519 identity public key, 64 hex chars
	Vault  []byte `json:"vault"`
}

type vaultRequest struct {
	Vault []byte `json:"vault"`
}

type userResponse struct {
	Handle string `json:"handle"`
	PubKey string `json:"pubkey"`
}

type createGroupRequest struct {
	GID string `json:"gid"` // 16 hex chars
	Tok string `json:"tok"` // 64 hex chars, derived from the group secret
}

type joinRequest struct {
	Tok   string `json:"tok"`
	Since uint64 `json:"since"` // only send messages with a higher sequence number
}

type sendRequest struct {
	CT []byte `json:"ct"`
}

// event is what GET /api/events pushes: a message ("msg") or a direct-chat notice ("dm").
type event struct {
	Type   string `json:"t"`
	GID    string `json:"gid,omitempty"`
	CT     []byte `json:"ct,omitempty"`
	Seq    uint64 `json:"seq,omitempty"`
	TS     int64  `json:"ts,omitempty"`
	Handle string `json:"handle,omitempty"` // dm: who wants to chat
}

type joinResponse struct {
	Messages []event `json:"messages"`
}

type fileResponse struct {
	ID   string `json:"id"`
	Size int64  `json:"size"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type inviteRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"` // the group's invite code
}

type gidRequest struct {
	Group int64  `json:"group"`
	GID   string `json:"gid"`
}

type grantRequest struct {
	Group  int64  `json:"group"`
	Handle string `json:"handle"`
	Sealed []byte `json:"sealed"`
}
