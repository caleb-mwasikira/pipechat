package client

import (
	"fmt"
	"regexp"
	"strings"

	"pipechat/internal/crypto"
)

// Public (open) groups, and closing chats you do not want in the sidebar.

var openNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{3,24}$`)

// What happens with the answers of the server.
type openChecked struct {
	name   string
	taken  bool
	secret *crypto.Secret
	err    error
}
type openFound struct {
	name  string
	group OpenGroup
	err   error
}
type openListed struct {
	groups []OpenGroup
	err    error
}
type openUnlisted struct {
	name string
	err  error
}

// createCommand handles /create NAME and /create NAME -open.
func (m *Model) createCommand(current *groupState, argument string) {
	open := false
	for _, flag := range []string{"-open", "--open"} {
		if strings.HasSuffix(argument, " "+flag) || argument == flag {
			argument, open = strings.TrimSpace(strings.TrimSuffix(argument, flag)), true
		}
	}
	secret, err := crypto.NewSecret(cleanText(argument))
	if err != nil {
		m.notice(current, err.Error())
		return
	}
	if !open {
		m.addGroup(secret, false)
		return
	}
	if !openNamePattern.MatchString(secret.Name) {
		m.notice(current, "a public group's name is 3-24 characters: letters, digits, - or _ (it is what people type after /join)")
		return
	}
	// A public name must be free before the group is made.
	server := m.server
	go func() {
		_, err := server.GetOpenGroup(secret.Name)
		switch {
		case err == nil:
			server.report(openChecked{name: secret.Name, taken: true})
		case isNotFound(err):
			server.report(openChecked{name: secret.Name, secret: secret})
		default:
			server.report(openChecked{name: secret.Name, err: err})
		}
	}()
}

// openChecked: the name is free, so make the group and, once the server has it, list it.
func (m *Model) startOpenGroup(result openChecked) {
	current := m.groups[m.current]
	switch {
	case result.err != nil:
		m.notice(current, "could not check the name: "+result.err.Error())
	case result.taken:
		m.notice(current, "a public group called "+result.name+" already exists; pick another name")
	default:
		m.addGroup(result.secret, false)
		m.groups[len(m.groups)-1].openName = result.name
		m.notice(m.groups[len(m.groups)-1], "This group is public: anyone can join it with /join "+result.name+". The server can read public groups.")
	}
}

// joinByName handles /join NAME for a public group.
func (m *Model) joinByName(current *groupState, name string) {
	server := m.server
	go func() {
		group, err := server.GetOpenGroup(name)
		server.report(openFound{name: name, group: group, err: err})
	}()
	m.notice(current, "looking for the public group "+name+"...")
}

func (m *Model) openFound(result openFound) {
	current := m.groups[m.current]
	if result.err != nil {
		if isNotFound(result.err) {
			m.notice(current, "no public group called "+result.name+"; /join also takes an invite code, and /public lists the open groups")
		} else {
			m.notice(current, "could not look up "+result.name+": "+result.err.Error())
		}
		return
	}
	secret, err := crypto.ParseCode(result.group.Code)
	if err != nil {
		m.notice(current, "that public group is broken: "+err.Error())
		return
	}
	if m.findGroup(secret.GIDHex()) != nil {
		m.notice(current, "you are already in "+result.group.Name)
		return
	}
	m.addGroup(secret, true)
}

// showOpenGroups prints the public groups.
func (m *Model) showOpenGroups(result openListed) {
	current := m.groups[m.current]
	if result.err != nil {
		m.notice(current, "could not get the public groups: "+result.err.Error())
		return
	}
	if len(result.groups) == 0 {
		m.notice(current, "no public groups yet; make one with /create NAME -open")
		return
	}
	m.notice(current, fmt.Sprintf("Public groups (%d). Join one with /join NAME:", len(result.groups)))
	for _, group := range result.groups {
		m.notice(current, fmt.Sprintf("  %-24s by @%s", group.Name, group.Creator))
	}
}

// closedChat is a chat taken out of the sidebar, kept so it can be brought back.
type closedChat struct {
	Name string `json:"name"`
	Code string `json:"code,omitempty"` // a group's invite code
	Peer string `json:"peer,omitempty"` // a direct chat's other person
}

// closeCommand handles /close: the chat leaves the sidebar, but nothing is deleted. A group stays a
// group you belong to and its messages stay on this computer; /reopen brings it back.
func (m *Model) closeCommand(current *groupState) {
	if current.secret == nil {
		m.notice(current, "this is the welcome page; open a chat first, then /close it")
		return
	}
	if current.adding {
		m.notice(current, "wait a moment, this chat is still being set up")
		return
	}
	id := current.secret.GIDHex()
	closed := closedChat{Name: current.secret.Name}
	if current.direct {
		closed = closedChat{Name: "@" + current.peer, Peer: current.peer}
		kept := []string{}
		for _, peer := range m.vault.Direct {
			if peer != current.peer {
				kept = append(kept, peer)
			}
		}
		m.vault.Direct = kept
	} else {
		closed.Code = current.secret.Code()
	}
	m.vault.Closed = append(m.vault.Closed, closed)
	m.server.enqueue(func() { m.server.LeaveGroup(id) }) // stop receiving it live
	m.removeGroup(current)
	m.saveVault()
	m.refreshSuggestions()
	m.switchTo(m.current)
	m.notice(m.groups[m.current], "Closed "+closed.Name+". /closed lists closed chats and /reopen NAME brings one back.")
}

// closedCommand handles /closed.
func (m *Model) closedCommand(current *groupState) {
	if len(m.vault.Closed) == 0 {
		m.notice(current, "no closed chats")
		return
	}
	m.notice(current, "Closed chats; bring one back with /reopen NAME:")
	for _, chat := range m.vault.Closed {
		m.notice(current, "  "+chat.Name)
	}
}

// reopenCommand handles /reopen NAME.
func (m *Model) reopenCommand(current *groupState, argument string) {
	name := strings.TrimSpace(argument)
	if name == "" {
		m.closedCommand(current)
		return
	}
	for index, chat := range m.vault.Closed {
		if !strings.EqualFold(chat.Name, name) && !strings.EqualFold(chat.Name, "@"+strings.TrimPrefix(name, "@")) {
			continue
		}
		m.vault.Closed = append(m.vault.Closed[:index:index], m.vault.Closed[index+1:]...)
		m.refreshSuggestions()
		if chat.Peer != "" {
			m.startDirect(chat.Peer, true)
			return
		}
		secret, err := crypto.ParseCode(chat.Code)
		if err != nil {
			m.notice(current, "that chat's code is damaged: "+err.Error())
			return
		}
		g := &groupState{secret: secret, adding: true}
		m.groups = append(m.groups, g)
		m.joinGroupAsync(g)
		m.replayHistory(g)
		m.switchTo(len(m.groups) - 1)
		return
	}
	m.notice(current, "no closed chat called "+name+", see /closed")
}

// forgetClosed drops a chat from the closed list when it comes back some other way (a new /dm, a join).
func (m *Model) forgetClosed(match func(closedChat) bool) {
	kept := m.vault.Closed[:0:0]
	for _, chat := range m.vault.Closed {
		if !match(chat) {
			kept = append(kept, chat)
		}
	}
	m.vault.Closed = kept
	m.refreshSuggestions()
}

func isNotFound(err error) bool {
	var failure *apiError
	return asAPIError(err, &failure) && failure.Status == 404
}
