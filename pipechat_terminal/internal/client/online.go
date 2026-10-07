package client

import (
	"pipechat/internal/client/ui"
)

type onlineResult struct {
	groupID string
	handles []string
	err     error
}

// fetchOnline asks who has the current chat open. The answer is shown in the sidebar.
func (m *Model) fetchOnline() {
	g := m.groups[m.current]
	if g.secret == nil || g.adding || !m.loggedIn || m.server == nil {
		return
	}
	id, server := g.secret.GIDHex(), m.server
	go func() {
		handles, err := server.Online(id)
		server.report(onlineResult{groupID: id, handles: handles, err: err})
	}()
}

// onlinePanel is the list under the chats in the sidebar: who is here right now.
func (m *Model) onlinePanel(room int) string {
	g := m.groups[m.current]
	if g.secret == nil || m.session == nil {
		return ""
	}
	return ui.OnlinePanel(m.peersOnline[g.secret.GIDHex()], m.session.handle, m.connected, room)
}
