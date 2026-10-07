package client

import (
	"crypto/ecdh"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"pipechat/internal/client/music"
	"pipechat/internal/client/ui"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"pipechat/internal/client/files"
	"pipechat/internal/crypto"
)

const maxMessageLen = 2000

// A line shown in a group's message list.
type entry struct {
	when   time.Time
	handle string // empty for system notices
	text   string
	self   bool
	seq    uint64 // server sequence number, 0 for notices
	raw    bool   // already styled: show as it is, with no time stamp
}

type groupState struct {
	secret    *crypto.Secret // nil for the welcome page
	entries   []entry
	unread    int
	adding    bool // just created or joined here, waiting for the server to agree
	announced bool // whether to tell the group we arrived once the server agrees
	direct    bool // a two-person chat; peer is the other handle
	peer      string
	notify    bool            // direct: tell the peer once the server agrees
	seen      map[uint64]bool // sequence numbers already shown
	webID     int64           // the website group this came from, 0 if none
	openName  string          // set on a group just made public: list it under this name once created
	webGrant  bool            // true when the key came from another member (confirm it), false when we created it
}

// What the account's sealed vault holds. The server cannot read any of it.
type vaultData struct {
	PrivateKey string                `json:"private_key"`      // hex X25519 identity key
	Groups     []string              `json:"groups"`           // invite codes
	Direct     []string              `json:"direct"`           // handles of direct chats
	Pins       map[string]string     `json:"pins"`             // handle -> public key first seen for it
	Files      map[string]fileRecord `json:"files"`            // short name -> shared file
	Closed     []closedChat          `json:"closed,omitempty"` // chats taken out of the sidebar with /close
}

// What goes inside a sealed message.
type payload struct {
	Kind   string `json:"k"` // "msg", "join" or "file"
	Handle string `json:"h"`
	Body   string `json:"b,omitempty"`
	Name   string `json:"n,omitempty"` // file: short name
	Link   string `json:"l,omitempty"` // file: download link, with the key in it
	Size   int64  `json:"s,omitempty"` // file: bytes
}

// A shared file we know about: our own uploads and what others sent us.
type fileRecord struct {
	Link string `json:"link"`
	Size int64  `json:"size"`
	From string `json:"from"`
	At   int64  `json:"at"` // unix seconds
}

const filesLimit = 200

type pendingAuth struct {
	email    string
	password string // kept for silent logins after the connection drops
	handle   string // the chat handle; the server tells us at login
	account  crypto.Account
}

// Results of requests, reported by the goroutine that made them.
type authResult struct {
	auth      *pendingAuth
	login     LoginResult
	err       error
	reconnect bool // a silent login after the connection was lost
}
type createResult struct {
	group *groupState
	err   error
}
type joinResult struct {
	group    *groupState
	messages []Event
	err      error
}
type lookupResult struct {
	peer      string
	publicKey string
	err       error
}
type requestFailed struct {
	what  string
	group *groupState
	err   error
}

type profileResult struct {
	profile Profile
	err     error
}

type bioSaved struct {
	bio string
	err error
}

type kickResult struct {
	group  *groupState
	handle string
	err    error
}

type inviteResult struct {
	to  string
	err error
}

type Model struct {
	store *Store

	// The server. server is nil until we know an address; serverReached turns true
	// when it first answers, and until then the connect screen is shown.
	server        *Server
	startURL      string        // the address to connect to at start, if we already know one
	report        func(tea.Msg) // how the server reaches this screen (the program's Send)
	serverReached bool
	connectError  string
	connectInput  textinput.Model
	online        bool

	// Login screen.
	emailInput    textinput.Model
	passwordInput textinput.Model
	focus         int
	authError     string
	pending       *pendingAuth

	// Chat screen. session is nil until logged in.
	session       *pendingAuth
	vault         vaultData
	identity      *ecdh.PrivateKey
	loggedIn      bool // logged in on the server right now
	connected     bool // the WebSocket is open
	relogging     bool
	pendingNotice string
	syncing       bool
	peersOnline   map[string][]string // chat id -> handles with it open, as last asked
	playing       *nowPlaying         // the song being played, if any
	loadingMusic  bool                // a track is being fetched
	list          music.Playlist      // what plays now and next, with shuffle and repeat
	proxy         *music.Proxy        // lets a player stream a song without the login token
	showHelp      bool                // a full-screen page (help, /whois) is open
	pageTitle     string
	progress      *files.Progress // how far the upload or download has got
	pathMode      bool            // the input holds /upload or /download, whose suggestions are file names
	pageFooter    string          // what the bottom line of a page says; empty for the usual
	pendingSend   *pendingUpload  // an upload waiting for a yes on its preview page
	pageContent   func(width int) string
	helpView      viewport.Model
	syncBusy      map[string]time.Time // website group work already under way, and since when
	uploadTarget  *groupState
	busy          bool          // an upload or download is running
	transfer      string        // progress text for the header
	groups        []*groupState // groups[0] is the welcome page
	current       int
	input         textinput.Model
	pendingDirect map[string]bool // handle -> whether we started it (and so must notify them)
	viewport      viewport.Model
	width         int
	height        int
}

// NewModel makes the screen. address is the server to use, or empty to use the
// one from last time. With neither, the screen asks the user to /connect.
func NewModel(store *Store, address string) *Model {
	newInput := func(placeholder string, secret bool) textinput.Model {
		input := textinput.New()
		input.Placeholder = placeholder
		input.Prompt = "> "
		input.CharLimit = maxMessageLen
		if secret {
			input.EchoMode = textinput.EchoPassword
		}
		return input
	}
	m := &Model{
		store:         store,
		connectInput:  newInput("/connect pipechat.kibandaskimart.com", false),
		emailInput:    newInput("email you registered with", false),
		passwordInput: newInput("password", true),
		input:         newInput("message or /command", false),
		viewport:      viewport.New(10, 5),
		groups:        []*groupState{{}},
	}
	m.connectInput.Focus()
	m.input.ShowSuggestions = true
	m.input.CompletionStyle = ui.Dim
	m.input.KeyMap.AcceptSuggestion = key.NewBinding(key.WithKeys("tab", "right"))
	m.input.KeyMap.NextSuggestion = key.NewBinding(key.WithKeys("down"))
	m.input.KeyMap.PrevSuggestion = key.NewBinding(key.WithKeys("up"))
	m.refreshSuggestions()
	m.showLogin()

	if address == "" {
		address = store.ServerURL
	}
	if address != "" {
		if url, err := ParseServerURL(address); err == nil {
			m.startURL = url
		} else {
			m.connectError = err.Error()
		}
	}
	return m
}

// Connect is called once the program exists, so the server has somewhere to send
// its events. It starts connecting to the server we already know, if any.
func (m *Model) Connect(report func(tea.Msg)) {
	m.report = report
	if m.startURL != "" {
		m.connectTo(m.startURL)
	}
}

// connectTo drops any earlier server and starts connecting to address.
func (m *Model) connectTo(address string) {
	if m.server != nil {
		m.server.Stop()
	}
	m.connectError = ""
	m.serverReached = false
	m.server = NewServer(address)
	m.server.Connect(m.report)
}

// showLogin sets up the login screen, with the last email pre-filled.
func (m *Model) showLogin() {
	m.emailInput.SetValue(m.store.LastEmail)
	m.focusField(0)
	if m.store.LastEmail != "" {
		m.focusField(1)
	}
}

// serverAnswered runs the first time the server answers. It is remembered for next time.
func (m *Model) serverAnswered() {
	m.serverReached = true
	m.connectError = ""
	if m.store.ServerURL != m.server.URL() {
		// A different server has different accounts.
		m.store.ServerURL = m.server.URL()
		m.store.LastEmail = ""
		m.showLogin()
	}
	m.store.Save()
}

func (m *Model) Init() tea.Cmd { return tea.Batch(textinput.Blink, syncTicks()) }

// Event is how background goroutines (the link) reach the screen.
func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = message.Width, message.Height
		m.viewport.Width = max(10, m.width-ui.SidebarWidth-2)
		m.layout()
		m.input.Width = max(10, m.width-4)
		m.helpView.Width, m.helpView.Height = m.width, max(3, m.height-3)
		if m.showHelp {
			m.helpView.SetContent(m.pageContent(m.width))
		}
		m.refresh()
		return m, nil

	case ServerUp:
		m.online = true
		if !m.serverReached {
			m.serverAnswered()
		}
		if m.session != nil && !m.loggedIn && !m.relogging {
			m.relogging = true
			m.loginAsync(m.session, true)
		}
		return m, nil

	case ServerDown:
		m.online = false
		if !m.serverReached {
			m.connectError = "cannot reach " + m.server.URL() + ": " + shortError(message.Err)
		}
		return m, nil

	case SocketUp:
		m.connected = true
		return m, nil

	case SocketDown:
		// The session is gone with the socket; ServerUp logs in again when the server is back.
		m.connected, m.loggedIn, m.online = false, false, false
		return m, nil

	case SocketEvent:
		m.handleEvent(message.Event)
		return m, nil

	case authResult:
		m.handleAuth(message)
		return m, nil

	case syncTick:
		m.syncAsync()
		m.fetchOnline()
		return m, syncTicks()

	case onlineResult:
		if message.err == nil {
			if m.peersOnline == nil {
				m.peersOnline = map[string][]string{}
			}
			m.peersOnline[message.groupID] = message.handles
		}
		return m, nil

	case syncResult:
		m.handleSync(message)
		return m, nil

	case createResult:
		m.handleCreated(message)
		return m, nil

	case joinResult:
		m.handleJoined(message)
		return m, nil

	case lookupResult:
		m.handleLookup(message)
		return m, nil

	case requestFailed:
		target := message.group
		if target == nil || m.findGroupState(target) < 0 {
			target = m.groups[m.current]
		}
		m.notice(target, message.what+": "+message.err.Error())
		return m, nil

	case files.Progress:
		if m.busy { // a late tick after the transfer ended must not bring the bar back
			m.progress = &message
			m.transfer = message.Verb + " " + message.Name
			m.layout()
		}
		return m, nil

	case files.Done:
		m.finishUpload(message)
		return m, nil

	case files.Downloaded:
		m.busy, m.transfer, m.progress = false, "", nil
		m.layout()
		current := m.groups[m.current]
		if message.Err != nil {
			m.notice(current, "download failed: "+message.Err.Error())
		} else {
			m.notice(current, fmt.Sprintf("downloaded %s (%s) to %s: %s", message.Name, files.FormatSize(message.Bytes), message.Dir, strings.Join(message.Entries, ", ")))
		}
		return m, nil

	case musicList:
		m.showMusic(message)
		return m, nil

	case musicStart:
		m.beginQueue(message)
		return m, nil

	case musicReady:
		m.startTrack(message)
		return m, nil

	case musicEnded:
		m.songEnded(message)
		return m, nil

	case openChecked:
		m.startOpenGroup(message)
		return m, nil

	case openFound:
		m.openFound(message)
		return m, nil

	case openListed:
		m.showOpenGroups(message)
		return m, nil

	case openUnlisted:
		if message.err != nil {
			m.notice(m.groups[m.current], "could not make "+message.name+" private: "+message.err.Error())
		} else {
			m.notice(m.groups[m.current], message.name+" is no longer listed. People already in it stay in; nobody new can join by name.")
		}
		return m, nil

	case profileResult:
		return m, m.showProfile(message)

	case bioSaved:
		current := m.groups[m.current]
		switch {
		case message.err != nil:
			m.notice(current, "could not save your bio: "+message.err.Error())
		case message.bio == "":
			m.notice(current, "your bio is cleared")
		default:
			m.notice(current, "your bio is saved; people see it with /whois @"+m.session.handle)
		}
		return m, nil

	case kickResult:
		target := message.group
		if m.findGroupState(target) < 0 {
			target = m.groups[m.current]
		}
		if message.err != nil {
			m.notice(target, "could not remove @"+message.handle+": "+message.err.Error())
		} else {
			m.notice(target, "removed @"+message.handle+" from the group. They can no longer read or send messages in it.")
		}
		return m, nil

	case inviteResult:
		if message.err != nil {
			m.notice(m.groups[m.current], "could not email "+message.to+": "+message.err.Error()+" (you can still share /code yourself)")
		} else {
			m.notice(m.groups[m.current], "invitation sent to "+message.to)
		}
		return m, nil

	case tea.KeyMsg:
		if message.String() == "ctrl+c" {
			m.stopMusic()
			return m, tea.Quit
		}
		switch {
		case m.session != nil:
			return m.updateChat(message)
		case !m.serverReached:
			return m.updateConnect(message)
		default:
			return m.updateAuth(message)
		}
	}
	return m, nil
}

// ---- login screen ----

func (m *Model) fields() []*textinput.Model {
	return []*textinput.Model{&m.emailInput, &m.passwordInput}
}

func (m *Model) focusField(index int) {
	m.focus = index
	for position, field := range m.fields() {
		if position == index {
			field.Focus()
		} else {
			field.Blur()
		}
	}
}

func (m *Model) updateAuth(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+s": // change server
		m.server.Stop()
		m.server, m.serverReached, m.online, m.pending = nil, false, false, nil
		m.connectInput.Focus()
		return m, nil
	case "tab", "down":
		m.focusField((m.focus + 1) % len(m.fields()))
		return m, nil
	case "shift+tab", "up":
		m.focusField((m.focus + len(m.fields()) - 1) % len(m.fields()))
		return m, nil
	case "enter":
		if m.focus < len(m.fields())-1 {
			m.focusField(m.focus + 1)
			return m, nil
		}
		m.submitAuth()
		return m, nil
	}
	var command tea.Cmd
	*m.fields()[m.focus], command = m.fields()[m.focus].Update(key)
	return m, command
}

func (m *Model) submitAuth() {
	email := strings.ToLower(strings.TrimSpace(m.emailInput.Value()))
	password := m.passwordInput.Value()
	switch {
	case m.pending != nil:
		return
	case !m.online:
		m.authError = "not connected to the server yet"
		return
	case !strings.Contains(email, "@") || password == "":
		m.authError = "enter the email and password you registered with on the website"
		return
	}
	m.authError = ""
	m.pending = &pendingAuth{email: email, password: password, account: crypto.DeriveAccount(email, password)}
	m.loginAsync(m.pending, false)
}

func (m *Model) loginAsync(auth *pendingAuth, reconnect bool) {
	go func() {
		result, err := m.server.Login(auth.email, auth.password)
		m.server.report(authResult{auth: auth, login: result, err: err, reconnect: reconnect})
	}()
}

func (m *Model) authView() string {
	title := "Log in to Pipechat"
	hint := "Enter to log in · Ctrl+S to change server · Ctrl+C to quit\nNo account yet? Register on the website."
	lines := []string{ui.Title.Render(title), "", "Email", m.emailInput.View(), "", "Password", m.passwordInput.View(), ""}
	switch {
	case m.pending != nil:
		lines = append(lines, ui.Dim.Render("working..."))
	case m.authError != "":
		lines = append(lines, ui.Error.Render(m.authError))
	case !m.online:
		lines = append(lines, ui.Dim.Render("connecting to "+m.server.URL()+"..."))
	default:
		lines = append(lines, ui.Dim.Render("connected to "+m.server.URL()))
	}
	lines = append(lines, ui.Dim.Render(hint))
	box := ui.AuthBox.Render(strings.Join(lines, "\n"))
	return lipgloss.Place(max(m.width, 1), max(m.height, 1), lipgloss.Center, lipgloss.Center, box)
}

// ---- results of requests, and events from the server ----

func (m *Model) handleAuth(result authResult) {
	if result.reconnect {
		m.relogging = false
		if result.err != nil {
			m.notice(m.groups[m.current], "could not log in again: "+result.err.Error())
			return
		}
		m.loggedIn = true
		m.afterLogin(result.login.Peers)
		return
	}

	if result.auth != m.pending {
		return // from a server we have since left
	}
	m.pending = nil
	if result.err != nil {
		m.authError = result.err.Error()
		return
	}
	m.session = result.auth
	m.session.handle = result.login.Handle
	if len(result.login.Vault) > 0 {
		m.loadVault(result.login.Vault)
	}
	fresh := m.identity == nil
	if fresh {
		// First login, or the vault cannot be opened (the website password was reset, so the
		// vault key changed). Start with new keys; groups come back from the website.
		m.newIdentity(len(result.login.Vault) > 0)
		if m.identity == nil {
			m.session = nil
			m.server.Logout()
			m.authError = "could not make new keys"
			return
		}
	}
	m.loggedIn = true
	m.store.LastEmail = m.session.email
	m.store.Save()
	m.input.Focus()
	for _, line := range ui.WelcomeLines(m.session.handle, crypto.Fingerprint(hex.EncodeToString(m.identity.PublicKey().Bytes()))) {
		m.groups[0].entries = append(m.groups[0].entries, entry{text: line, raw: true})
	}
	if m.pendingNotice != "" {
		m.notice(m.groups[0], m.pendingNotice)
		m.pendingNotice = ""
	}
	m.refresh()
	for _, peer := range m.vault.Direct {
		m.startDirect(peer, false)
	}
	m.afterLogin(result.login.Peers)
	m.syncAsync()
}

// newIdentity makes a new key pair and an empty vault, and sends both to the server.
func (m *Model) newIdentity(replacing bool) {
	identity, err := crypto.NewIdentity()
	if err != nil {
		return
	}
	m.identity = identity
	m.vault = vaultData{PrivateKey: hex.EncodeToString(identity.Bytes()), Pins: map[string]string{}}
	m.groups = m.groups[:1]
	plain, _ := json.Marshal(m.vault)
	sealed, err := crypto.SealVault(m.session.account.VaultKey, m.session.handle, plain)
	if err != nil {
		m.identity = nil
		return
	}
	publicKey := hex.EncodeToString(identity.PublicKey().Bytes())
	m.server.enqueue(func() {
		if err := m.server.SaveIdentity(publicKey, sealed); err != nil {
			m.server.report(requestFailed{what: "could not save your new keys", err: err})
		}
	})
	if replacing {
		m.pendingNotice = "Your saved keys could not be opened (was your password reset?), so you have new keys. Your groups return as other members come online."
	}
}

// afterLogin joins the groups again and opens direct chats that others started.
func (m *Model) afterLogin(peers []string) {
	for _, peer := range peers {
		m.startDirect(peer, false)
	}
	for _, g := range m.groups[1:] {
		if !g.adding {
			m.joinGroupAsync(g)
		}
	}
}

func (m *Model) handleEvent(event Event) {
	switch event.Type {
	case "msg":
		m.receive(event)
	case "dm":
		m.startDirect(event.Handle, false)
	case "kick":
		if g := m.findGroup(event.GID); g != nil {
			m.droppedFromGroup(g)
		}
	}
	m.refresh()
}

func (m *Model) handleCreated(result createResult) {
	g := result.group
	switch {
	case result.err == nil:
		m.finishAdding(g)
	case g.direct && g.adding && Conflict(result.err):
		// The other person opened this chat first: join it instead.
		m.joinGroupAsync(g)
	case g.adding:
		m.removeGroup(g)
		m.notice(m.groups[m.current], "could not add group: "+result.err.Error())
	default:
		m.notice(g, "create: "+result.err.Error())
	}
	m.refresh()
}

func (m *Model) handleJoined(result joinResult) {
	g := result.group
	if m.findGroupState(g) < 0 {
		return // left in the meantime
	}
	if result.err != nil {
		var failure *apiError
		if errors.As(result.err, &failure) && failure.Status == http.StatusForbidden && strings.Contains(failure.Message, "no longer a member") {
			if g.adding {
				m.removeGroup(g)
				m.notice(m.groups[m.current], "could not join "+g.secret.Name+": "+failure.Message)
			} else {
				m.droppedFromGroup(g)
			}
			return
		}
		if g.adding {
			m.removeGroup(g)
			m.notice(m.groups[m.current], "could not add group: "+result.err.Error())
		} else {
			m.notice(g, "join: "+result.err.Error())
		}
		return
	}
	for _, event := range result.messages {
		m.receive(event)
	}
	if g.adding {
		m.finishAdding(g)
	}
	m.refresh()
}

// finishAdding runs once the server has accepted a group we created or joined.
func (m *Model) finishAdding(g *groupState) {
	g.adding = false
	if g.announced {
		m.send(g, payload{Kind: "join", Handle: m.session.handle})
	}
	if g.direct {
		m.vault.Direct = appendMissing(m.vault.Direct, g.peer)
		if g.notify {
			peer := g.peer
			m.server.enqueue(func() { m.server.NotifyDirect(peer) })
		}
	}
	m.saveVault()
	if g.openName != "" {
		name, code, server := g.openName, g.secret.Code(), m.server
		g.openName = ""
		m.server.enqueue(func() {
			if err := server.ListOpenGroup(name, code); err != nil {
				server.report(requestFailed{what: "could not make " + name + " public (it stays private)", err: err})
			}
		})
	}
	if g.webID != 0 {
		id, groupID, granted := g.webID, g.secret.GIDHex(), g.webGrant
		m.server.enqueue(func() {
			var err error
			if granted {
				err = m.server.SyncGrantReceived(id)
			} else {
				err = m.server.SyncGroupCreated(id, groupID)
			}
			if err != nil {
				m.server.report(requestFailed{what: "could not tell the website about the group", err: err})
			}
		})
	}
}

func (m *Model) handleLookup(result lookupResult) {
	if result.err != nil {
		delete(m.pendingDirect, result.peer)
		m.notice(m.groups[m.current], "could not start a chat with @"+result.peer+": "+result.err.Error())
		return
	}
	m.openDirect(result.peer, result.publicKey)
}

// loadVault restores the identity key and the group list from the sealed vault.
func (m *Model) loadVault(sealed []byte) {
	plain, err := crypto.OpenVault(m.session.account.VaultKey, m.session.handle, sealed)
	var saved vaultData
	if err == nil {
		err = json.Unmarshal(plain, &saved)
	}
	var identity *ecdh.PrivateKey
	if err == nil {
		identity, err = crypto.ParsePrivateKey(saved.PrivateKey)
	}
	if err != nil {
		m.notice(m.groups[0], "could not open your saved keys and groups")
		return
	}
	m.vault = saved
	if m.vault.Pins == nil {
		m.vault.Pins = map[string]string{}
	}
	m.identity = identity
	for _, code := range saved.Groups {
		if secret, err := crypto.ParseCode(code); err == nil {
			group := &groupState{secret: secret}
			m.groups = append(m.groups, group)
			m.replayHistory(group)
		}
	}
}

func (m *Model) saveVault() {
	m.vault.Groups = []string{}
	for _, g := range m.groups[1:] {
		if !g.adding && !g.direct {
			m.vault.Groups = append(m.vault.Groups, g.secret.Code())
		}
	}
	plain, _ := json.Marshal(m.vault)
	sealed, err := crypto.SealVault(m.session.account.VaultKey, m.session.handle, plain)
	if err == nil {
		err = m.server.enqueue(func() {
			if err := m.server.SaveVault(sealed); err != nil {
				m.server.report(requestFailed{what: "could not save your keys and groups", err: err})
			}
		})
	}
	if err != nil {
		m.notice(m.groups[0], "could not save your keys and groups: "+err.Error())
	}
}

func appendMissing(list []string, item string) []string {
	for _, existing := range list {
		if existing == item {
			return list
		}
	}
	return append(list, item)
}

// seqKey names where the newest message seen in a group is remembered. It includes the
// handle because several people can use one computer, and a direct chat has the same id for
// both of them: what one person has read says nothing about what the other has.
func (m *Model) seqKey(groupID string) string {
	return m.session.handle + "/" + groupID
}

func (m *Model) joinGroupAsync(g *groupState) {
	id, token, since := g.secret.GIDHex(), g.secret.JoinToken(), m.store.History.Last(m.seqKey(g.secret.GIDHex()))
	if err := m.server.enqueue(func() {
		messages, err := m.server.JoinGroup(id, token, since)
		m.server.report(joinResult{group: g, messages: messages, err: err})
	}); err != nil {
		m.notice(g, "join: "+err.Error())
	}
}

func (m *Model) createGroupAsync(g *groupState) {
	id, token := g.secret.GIDHex(), g.secret.JoinToken()
	if err := m.server.enqueue(func() { m.server.report(createResult{group: g, err: m.server.CreateGroup(id, token)}) }); err != nil {
		m.notice(g, "create: "+err.Error())
	}
}

// startDirect asks the server for a user's public key. The chat opens when the answer arrives.
func (m *Model) startDirect(peer string, notify bool) {
	for _, g := range m.groups[1:] {
		if g.direct && g.peer == peer {
			return
		}
	}
	if m.pendingDirect == nil {
		m.pendingDirect = map[string]bool{}
	}
	if _, waiting := m.pendingDirect[peer]; waiting {
		return
	}
	m.forgetClosed(func(c closedChat) bool { return c.Peer == peer })
	m.pendingDirect[peer] = notify
	m.server.enqueue(func() {
		publicKey, err := m.server.LookupUser(peer)
		m.server.report(lookupResult{peer: peer, publicKey: publicKey, err: err})
	})
}

// openDirect finishes startDirect: check the peer's key against the one pinned
// earlier, derive the shared secret and create or join the chat.
func (m *Model) openDirect(peer string, publicKey string) {
	notify, waiting := m.pendingDirect[peer]
	if !waiting {
		return // a lookup from /whois
	}
	delete(m.pendingDirect, peer)

	current := m.groups[m.current]
	if pinned, known := m.vault.Pins[peer]; known && pinned != publicKey {
		m.notice(current, "WARNING: the key of @"+peer+" changed. Refusing to chat; ask them to confirm with /whois.")
		return
	}
	theirPublic, err := crypto.ParsePublicKey(publicKey)
	if err != nil {
		m.notice(current, "@"+peer+" has an invalid key")
		return
	}
	secret, err := crypto.DirectSecret(m.identity, m.session.handle, theirPublic, peer)
	if err != nil {
		m.notice(current, "could not start the chat: "+err.Error())
		return
	}
	if m.vault.Pins == nil {
		m.vault.Pins = map[string]string{}
	}
	_, pinnedBefore := m.vault.Pins[peer]
	m.vault.Pins[peer] = publicKey

	g := &groupState{secret: secret, adding: true, direct: true, peer: peer, notify: notify}
	m.groups = append(m.groups, g)
	m.replayHistory(g)
	m.createGroupAsync(g)
	if notify {
		m.switchTo(len(m.groups) - 1)
	}
	if !pinnedBefore {
		m.notice(g, "First chat with @"+peer+". Their key fingerprint is "+crypto.Fingerprint(publicKey)+"; compare it with theirs (/whois) to be sure it is them.")
	}
}

func (m *Model) findGroup(id string) *groupState {
	for _, g := range m.groups[1:] {
		if g.secret.GIDHex() == id {
			return g
		}
	}
	return nil
}

// droppedFromGroup forgets a group the creator removed us from.
// openPage takes over the whole screen with some text, until the person presses Esc.
func (m *Model) openPage(title string, content func(width int) string) tea.Cmd {
	if m.pendingSend == nil {
		m.pageFooter = ""
	}
	m.showHelp, m.pageTitle, m.pageContent = true, title, content
	m.helpView.Width, m.helpView.Height = m.width, max(3, m.height-3)
	m.helpView.SetContent(content(m.width))
	m.helpView.GotoTop()
	return tea.ClearScreen
}

// lookupProfile asks the server about a user; the answer opens their page.
func (m *Model) lookupProfile(handle string) {
	server := m.server
	go func() {
		profile, err := server.LookupProfile(handle)
		if err != nil {
			profile.Handle = handle
		}
		server.report(profileResult{profile: profile, err: err})
	}()
}

// showProfile opens the page for a /whois answer.
func (m *Model) showProfile(result profileResult) tea.Cmd {
	if result.err != nil {
		m.notice(m.groups[m.current], "no user @"+result.profile.Handle+" ("+result.err.Error()+")")
		return nil
	}
	p := result.profile
	own := p.Handle == m.session.handle
	keyCheck := ""
	if !own {
		switch pinned, known := m.vault.Pins[p.Handle]; {
		case known && pinned != p.PubKey:
			keyCheck = "WARNING: this key is not the one you saved when you first chatted. Do not trust this account until you have checked with them."
		case known:
			keyCheck = "matches the key you saved when you first chatted"
		default:
			keyCheck = "not saved yet; it is saved when you first chat, and compared every time after"
		}
	}
	fingerprint := crypto.Fingerprint(p.PubKey)
	return m.openPage("whois @"+p.Handle, func(width int) string {
		return ui.ProfilePage(width, ui.Profile{Handle: p.Handle, Name: p.Name, Bio: p.Bio, Joined: p.Joined, Online: p.Online}, own, fingerprint, keyCheck)
	})
}

func (m *Model) droppedFromGroup(g *groupState) {
	name := g.secret.Name
	m.store.History.Forget(m.seqKey(g.secret.GIDHex()))
	m.removeGroup(g)
	m.saveVault()
	m.notice(m.groups[0], "You were removed from the group "+name+" by its creator.")
	m.switchTo(m.current)
}

func (m *Model) removeGroup(target *groupState) {
	for index, g := range m.groups {
		if g == target {
			m.groups = append(m.groups[:index], m.groups[index+1:]...)
			if m.current >= len(m.groups) || m.current >= index && m.current > 0 {
				m.current--
			}
			return
		}
	}
}

func (m *Model) receive(frame Event) { m.receiveMessage(frame, false) }

// replayHistory shows a chat's messages saved by an earlier run.
func (m *Model) replayHistory(g *groupState) {
	key := m.seqKey(g.secret.GIDHex())
	for _, saved := range m.store.History.Messages(key) {
		m.receiveMessage(Event{Type: "msg", GID: g.secret.GIDHex(), CT: saved.CT, Seq: saved.Seq, TS: saved.TS}, true)
	}
}

// receiveMessage shows one message. replay is true for one saved earlier: it is not saved again,
// does not count as unread, and does not touch the list of shared files.
func (m *Model) receiveMessage(frame Event, replay bool) {
	g := m.findGroup(frame.GID)
	if g == nil || g.seen[frame.Seq] {
		return
	}
	if g.seen == nil {
		g.seen = map[uint64]bool{}
	}
	g.seen[frame.Seq] = true
	if !replay {
		m.store.History.Add(m.seqKey(frame.GID), frame.Seq, frame.TS, frame.CT)
	}

	line := entry{when: time.UnixMilli(frame.TS), seq: frame.Seq}
	plain, err := g.secret.Open(frame.CT)
	var content payload
	if err != nil || json.Unmarshal(plain, &content) != nil {
		line.text = "[message that could not be decrypted]"
	} else {
		line.handle = cleanText(content.Handle)
		line.self = line.handle == m.session.handle
		line.text = cleanText(content.Body)
		if content.Kind == "file" {
			line.handle = ""
			if !files.ShortNamePattern.MatchString(content.Name) {
				line.text = "[invalid file share]"
			} else {
				line.text = fmt.Sprintf("@%s shared \"%s\" (%s). Download with: /download %s DIR", cleanText(content.Handle), content.Name, files.FormatSize(content.Size), content.Name)
				if !line.self && !replay {
					m.rememberFile(content.Name, fileRecord{Link: content.Link, Size: content.Size, From: cleanText(content.Handle), At: time.Now().Unix()})
				}
			}
		} else if content.Kind == "join" {
			line.handle = ""
			line.text = "@" + cleanText(content.Handle) + " joined the group"
		}
	}
	// Messages can arrive out of order (history after a live one), so keep them sorted.
	position := len(g.entries)
	for position > 0 && g.entries[position-1].seq > line.seq {
		position--
	}
	g.entries = append(g.entries, entry{})
	copy(g.entries[position+1:], g.entries[position:])
	g.entries[position] = line
	if !replay && g != m.groups[m.current] {
		g.unread++
	}
}

// cleanText removes control characters and bidi overrides, so a group member
// cannot send terminal escape sequences or disguise text.
func cleanText(text string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t':
			return ' '
		case unicode.IsControl(r), r >= 0x202a && r <= 0x202e, r >= 0x2066 && r <= 0x2069:
			return -1
		}
		return r
	}, text)
}

func (m *Model) notice(g *groupState, text string) {
	for _, part := range strings.Split(text, "\n") {
		g.entries = append(g.entries, entry{when: time.Now(), text: part})
	}
	m.refresh()
}

func (m *Model) send(g *groupState, content payload) error {
	plain, _ := json.Marshal(content)
	sealed, err := g.secret.Seal(plain)
	if err != nil {
		return err
	}
	if !m.loggedIn {
		return errors.New("not connected to the server")
	}
	id := g.secret.GIDHex()
	return m.server.enqueue(func() {
		if err := m.server.SendMessage(id, sealed); err != nil {
			m.server.report(requestFailed{what: "not sent", group: g, err: err})
		}
	})
}

// ---- chat screen ----

func (m *Model) updateChat(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.showHelp {
		if m.pendingSend != nil {
			// The upload preview: a yes uploads, anything that closes the page cancels.
			switch key.String() {
			case "enter", "y", "Y":
				pending := m.pendingSend
				m.showHelp, m.pendingSend, m.pageFooter = false, nil, ""
				return m, tea.Batch(tea.ClearScreen, m.startUpload(pending.target, pending.argument))
			case "esc", "n", "N", "q":
				m.notice(m.pendingSend.target, "upload cancelled")
				m.showHelp, m.pendingSend, m.pageFooter = false, nil, ""
				return m, tea.ClearScreen
			}
			var command tea.Cmd
			m.helpView, command = m.helpView.Update(key)
			return m, command
		}
		switch key.String() {
		case "esc", "q", "enter", " ", "ctrl+h", "/":
			m.showHelp = false
			return m, tea.ClearScreen
		}
		var command tea.Cmd
		m.helpView, command = m.helpView.Update(key)
		return m, command
	}
	switch key.String() {
	case "tab", "ctrl+n":
		if key.String() == "tab" && m.suggestionShown() {
			break // Tab finishes the command being typed
		}
		m.switchTo((m.current + 1) % len(m.groups))
		return m, nil
	case "shift+tab", "ctrl+p":
		m.switchTo((m.current + len(m.groups) - 1) % len(m.groups))
		return m, nil
	case "pgup", "pgdown":
		var command tea.Cmd
		m.viewport, command = m.viewport.Update(key)
		return m, command
	case "enter":
		text := strings.TrimSpace(m.input.Value())
		m.input.Reset()
		if text == "" {
			return m, nil
		}
		if strings.HasPrefix(text, "/") {
			return m, m.runCommand(text)
		}
		g := m.groups[m.current]
		if g.secret == nil {
			m.notice(g, "join or create a group first: /create NAME or /join CODE")
		} else if err := m.send(g, payload{Kind: "msg", Handle: m.session.handle, Body: text}); err != nil {
			m.notice(g, "not sent: "+err.Error())
		}
		return m, nil
	}
	var command tea.Cmd
	m.input, command = m.input.Update(key)
	// File names depend on what has been typed, so work them out again after every key.
	if strings.HasPrefix(m.input.Value(), "/upload ") || strings.HasPrefix(m.input.Value(), "/download ") || m.pathMode {
		m.pathMode = strings.HasPrefix(m.input.Value(), "/upload ") || strings.HasPrefix(m.input.Value(), "/download ")
		m.refreshSuggestions()
	}
	return m, command
}

// barLines is how many lines the progress bar takes from the chat: one while a transfer is running.
func (m *Model) barLines() int {
	if m.progress != nil {
		return 1
	}
	return 0
}

// layout sizes the chat area to the window, leaving room for the progress bar when there is one.
func (m *Model) layout() {
	m.viewport.Height = max(3, m.height-4-m.barLines())
	m.refresh()
}

func (m *Model) switchTo(index int) {
	m.current = index
	m.groups[index].unread = 0
	m.fetchOnline()
	m.refresh()
}

func (m *Model) runCommand(text string) tea.Cmd {
	name, argument, _ := strings.Cut(text, " ")
	argument = strings.TrimSpace(argument)
	current := m.groups[m.current]

	switch name {
	case "/help":
		// A full-screen guide on a cleared screen; Esc brings the chat back.
		return m.openPage("help", ui.HelpGuide)

	case "/quit", "/exit":
		m.stopMusic()
		return tea.Quit

	case "/clear":
		// Only clears what is shown: the messages stay in the group for the others, and are not shown here again.
		current.entries = nil
		if current.secret != nil {
			m.store.History.Clear(m.seqKey(current.secret.GIDHex()))
		}
		m.refresh()

	case "/create":
		m.createCommand(current, argument)

	case "/join":
		secret, err := crypto.ParseCode(argument)
		if err != nil {
			// Not an invite code: perhaps the name of a public group.
			if openNamePattern.MatchString(argument) {
				m.joinByName(current, argument)
			} else {
				m.notice(current, "/join takes an invite code or the name of a public group (see /public)")
			}
			break
		}
		if m.findGroup(secret.GIDHex()) != nil {
			m.notice(current, "you are already in that group")
			break
		}
		m.addGroup(secret, true)

	case "/public":
		server := m.server
		go func() {
			groups, err := server.OpenGroups()
			server.report(openListed{groups: groups, err: err})
		}()

	case "/unlist":
		if !openNamePattern.MatchString(argument) {
			m.notice(current, "usage: /unlist NAME (a public group you made)")
			break
		}
		server := m.server
		go func() { server.report(openUnlisted{name: argument, err: server.UnlistOpenGroup(argument)}) }()

	case "/close":
		m.closeCommand(current)

	case "/closed":
		m.closedCommand(current)

	case "/reopen":
		m.reopenCommand(current, argument)

	case "/dm":
		peer := strings.ToLower(strings.TrimPrefix(argument, "@"))
		switch {
		case !crypto.ValidHandle(peer):
			m.notice(current, "usage: /dm HANDLE")
		case peer == m.session.handle:
			m.notice(current, "you cannot message yourself")
		default:
			for index, g := range m.groups {
				if g.direct && g.peer == peer {
					m.switchTo(index)
					return nil
				}
			}
			m.startDirect(peer, true)
		}

	case "/whois", "/me":
		peer := strings.ToLower(strings.TrimPrefix(argument, "@"))
		if name == "/me" || peer == "" {
			peer = m.session.handle
		}
		if !crypto.ValidHandle(peer) {
			m.notice(current, "usage: /whois HANDLE")
			break
		}
		m.lookupProfile(peer)

	case "/bio":
		switch {
		case argument == "":
			m.lookupProfile(m.session.handle)
		default:
			bio := argument
			if strings.EqualFold(argument, "clear") {
				bio = ""
			}
			server := m.server
			go func() { server.report(bioSaved{bio: bio, err: server.SetBio(bio)}) }()
		}

	case "/upload":
		return m.previewUpload(current, argument)

	case "/download":
		return m.startDownload(current, argument)

	case "/files":
		if len(m.vault.Files) == 0 {
			m.notice(current, "no shared files yet; use /upload PATH")
		}
		for name, record := range m.vault.Files {
			who := "you"
			if record.From != "" {
				who = "@" + record.From
			}
			m.notice(current, fmt.Sprintf("%s  %s  from %s  %s", name, files.FormatSize(record.Size), who, time.Unix(record.At, 0).Format("2006-01-02 15:04")))
		}

	case "/link":
		if record, found := m.vault.Files[argument]; found {
			m.notice(current, "Link for "+argument+" (it contains the decryption key, share it privately):\n"+record.Link)
		} else {
			m.notice(current, "no file named "+argument+", see /files")
		}

	case "/share":
		record, found := m.vault.Files[argument]
		switch {
		case !found:
			m.notice(current, "no file named "+argument+", see /files")
		case current.secret == nil:
			m.notice(current, "open a group or direct chat first")
		default:
			if err := m.send(current, payload{Kind: "file", Handle: m.session.handle, Name: argument, Link: record.Link, Size: record.Size}); err != nil {
				m.notice(current, "not sent: "+err.Error())
			} else {
				m.reportShare(current, argument, record.Size)
			}
		}

	case "/code":
		if current.direct {
			m.notice(current, "direct chats have no invite code")
			break
		}
		if current.secret == nil {
			m.notice(current, "select a group first")
			break
		}
		m.notice(current, "Invite code for "+current.secret.Name+" (anyone with it can read the group, share it privately):\n"+current.secret.Code())

	case "/invite":
		if current.direct {
			m.notice(current, "direct chats have no invite code")
			break
		}
		if current.secret == nil {
			m.notice(current, "select a group first")
			break
		}
		if argument == "" {
			m.notice(current, "usage: /invite EMAIL")
			break
		}
		if !m.loggedIn {
			m.notice(current, "not connected to the server")
			break
		}
		m.notice(current, "asking the server to email "+argument+" (the server sees the code when it does; /code keeps it private)...")
		code, to := current.secret.Code(), argument
		m.server.enqueue(func() { m.server.report(inviteResult{to: to, err: m.server.InviteByEmail(to, code)}) })

	case "/groups":
		for index, g := range m.groups[1:] {
			m.notice(current, fmt.Sprintf("%d. %s", index+1, g.secret.Name))
		}

	case "/switch":
		var index int
		if _, err := fmt.Sscanf(argument, "%d", &index); err != nil || index < 0 || index >= len(m.groups) {
			m.notice(current, "usage: /switch N (see /groups)")
			break
		}
		m.switchTo(index)

	case "/music":
		m.notice(current, "asking the server for the music library...")
		m.server.enqueue(func() {
			tracks, err := m.server.Music()
			m.server.report(musicList{tracks: tracks, err: err})
		})

	case "/play":
		m.playCommand(current, argument)

	case "/stop":
		if m.playing == nil && !m.list.Active() {
			m.notice(current, "nothing is playing")
		} else {
			m.notice(current, "stopped the music")
			m.stopMusic()
		}

	case "/next":
		m.skipCommand(current)

	case "/repeat":
		m.repeatCommand(current, strings.ToLower(argument))

	case "/shuffle":
		m.shuffleCommand(current, strings.ToLower(argument))

	case "/kick":
		handle := strings.ToLower(strings.TrimPrefix(argument, "@"))
		switch {
		case current.secret == nil || current.direct:
			m.notice(current, "open one of your groups first")
		case !crypto.ValidHandle(handle):
			m.notice(current, "usage: /kick HANDLE")
		case handle == m.session.handle:
			m.notice(current, "you cannot remove yourself; use /leave")
		default:
			id := current.secret.GIDHex()
			m.server.enqueue(func() {
				m.server.report(kickResult{group: current, handle: handle, err: m.server.KickMember(id, handle)})
			})
		}

	case "/leave":
		if current.secret == nil {
			m.notice(current, "nothing to leave here")
			break
		}
		id := current.secret.GIDHex()
		m.server.enqueue(func() { m.server.LeaveGroup(id) })
		m.store.History.Forget(m.seqKey(id))
		if current.direct {
			peer := current.peer
			m.server.enqueue(func() { m.server.ClearDirect(peer) })
			kept := []string{}
			for _, peer := range m.vault.Direct {
				if peer != current.peer {
					kept = append(kept, peer)
				}
			}
			m.vault.Direct = kept
		}
		m.removeGroup(current)
		m.saveVault()
		m.switchTo(m.current)

	default:
		m.notice(current, "unknown command, try /help")
	}
	return nil
}

// addGroup adds a group we created (joining false) or are joining with an invite code.
func (m *Model) addGroup(secret *crypto.Secret, joining bool) {
	if !m.loggedIn {
		m.notice(m.groups[m.current], "not connected to the server")
		return
	}
	m.forgetClosed(func(c closedChat) bool {
		return c.Code != "" && c.Code == secret.Code()
	})
	g := &groupState{secret: secret, adding: true, announced: joining}
	m.groups = append(m.groups, g)
	if joining {
		m.joinGroupAsync(g)
	} else {
		m.createGroupAsync(g)
	}
	m.switchTo(len(m.groups) - 1)
	m.notice(g, "Group "+secret.Name+". Invite others with /invite EMAIL or /code.")
}

func (m *Model) refresh() {
	if m.session == nil || m.width == 0 {
		return
	}
	wasAtBottom := m.viewport.AtBottom()
	var lines []string
	wrap := lipgloss.NewStyle().Width(m.viewport.Width)
	for _, line := range m.groups[m.current].entries {
		stamp := ui.Dim.Render(line.when.Format("15:04"))
		switch {
		case line.raw:
			lines = append(lines, line.text)
		case line.handle == "":
			lines = append(lines, wrap.Render(stamp+" "+ui.Dim.Render(line.text)))
		case line.self:
			lines = append(lines, wrap.Render(stamp+" "+ui.Self.Render("you")+": "+line.text))
		default:
			lines = append(lines, wrap.Render(stamp+" "+ui.Other.Render("@"+line.handle)+": "+line.text))
		}
	}
	m.viewport.SetContent(strings.Join(lines, "\n"))
	if wasAtBottom {
		m.viewport.GotoBottom()
	}
}

func (m *Model) View() string {
	switch {
	case m.session == nil && !m.serverReached:
		return m.connectView()
	case m.session == nil:
		return m.authView()
	}
	if m.showHelp {
		title := ui.Title.Render("Pipechat") + ui.Dim.Render("  "+m.pageTitle)
		footer := ui.Dim.Render("↑/↓ PgUp/PgDn scroll  ·  Esc, Enter or q to go back")
		if m.pageFooter != "" {
			footer = ui.Key.Render(m.pageFooter)
		}
		return title + "\n\n" + m.helpView.View() + "\n" + footer
	}
	status := "● online"
	if !m.connected {
		status = "○ reconnecting..."
	}
	if m.transfer != "" {
		status += "  ⇅ " + m.transfer
	}
	if m.playing != nil {
		status += "  ♪ " + m.playing.track.Title + m.list.Modes()
	} else if m.loadingMusic {
		status += "  ♪ loading..."
	}
	header := ui.Title.Render("Pipechat") + ui.Dim.Render("  @"+m.session.handle+"  "+status)

	var names []string
	for index, g := range m.groups {
		label := "welcome"
		if g.secret != nil {
			label = g.secret.Name
		}
		if g.unread > 0 {
			label += fmt.Sprintf(" (%d)", g.unread)
		}
		if index == m.current {
			names = append(names, ui.Active.Render("▸ "+label))
		} else {
			names = append(names, ui.Dim.Render("  "+label))
		}
	}
	list := strings.Join(names, "\n")
	if panel := m.onlinePanel(m.height - 2 - m.barLines() - len(names) - 3); panel != "" {
		list += "\n\n" + panel
	}
	sidebar := ui.Sidebar.Height(m.height - 2 - m.barLines()).Render(list)

	title := "welcome"
	if secret := m.groups[m.current].secret; secret != nil {
		title = secret.Name + ui.Dim.Render("  · end-to-end encrypted")
		if m.groups[m.current].direct {
			title += ui.Dim.Render(" · direct")
		}
	}
	chat := lipgloss.JoinVertical(lipgloss.Left, ui.Active.Render(title), m.viewport.View())
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, " "+strings.ReplaceAll(chat, "\n", "\n "))
	if m.progress != nil {
		return header + "\n" + body + "\n" + ui.TransferLine(*m.progress, m.width, time.Now()) + "\n" + m.input.View()
	}
	return header + "\n" + body + "\n" + m.input.View()
}

// rememberFile adds a shared file to the vault, dropping the oldest ones past the limit.
func (m *Model) rememberFile(name string, record fileRecord) {
	if m.vault.Files == nil {
		m.vault.Files = map[string]fileRecord{}
	}
	m.vault.Files[name] = record
	for len(m.vault.Files) > filesLimit {
		oldest := ""
		for candidate, existing := range m.vault.Files {
			if oldest == "" || existing.At < m.vault.Files[oldest].At {
				oldest = candidate
			}
		}
		delete(m.vault.Files, oldest)
	}
	m.saveVault()
}

// startUpload handles /upload PATH [-name SHORT]: it packs, encrypts and uploads in the background.
func (m *Model) startUpload(current *groupState, argument string) tea.Cmd {
	var source, name string
	arguments := files.SplitArguments(argument)
	for index := 0; index < len(arguments); index++ {
		switch {
		case (arguments[index] == "-name" || arguments[index] == "--name") && index+1 < len(arguments):
			index++
			name = arguments[index]
		case source == "":
			source = arguments[index]
		default:
			m.notice(current, "usage: /upload PATH [-name SHORTNAME]  (quote paths with spaces)")
			return nil
		}
	}
	if source == "" {
		m.notice(current, "usage: /upload PATH [-name SHORTNAME]")
		return nil
	}
	source = files.ExpandHome(source)
	info, err := os.Stat(source)
	if err != nil {
		m.notice(current, "cannot read "+source+": "+err.Error())
		return nil
	}
	if name == "" {
		absolute, _ := filepath.Abs(source)
		name = files.ShortName(filepath.Base(absolute))
	}
	switch {
	case !files.ShortNamePattern.MatchString(name):
		m.notice(current, "short name must be 1-40 characters: letters, digits, . _ -")
		return nil
	case m.busy:
		m.notice(current, "another transfer is still running")
		return nil
	case !m.loggedIn:
		m.notice(current, "not connected to the server")
		return nil
	}
	kind := "file"
	if info.IsDir() {
		kind = "folder"
	}
	m.busy, m.transfer, m.uploadTarget = true, "uploading "+name, current
	m.notice(current, "uploading "+kind+" "+source+" as \""+name+"\" (encrypted before it leaves this machine)...")
	return func() tea.Msg { return files.Upload(m.server, source, name) }
}

func (m *Model) finishUpload(result files.Done) {
	m.busy, m.transfer, m.progress = false, "", nil
	m.layout()
	target := m.uploadTarget
	m.uploadTarget = nil
	if target == nil || m.findGroupState(target) < 0 {
		target = m.groups[m.current]
	}
	if result.Err != nil {
		m.notice(target, "upload failed: "+result.Err.Error())
		return
	}
	m.rememberFile(result.Name, fileRecord{Link: result.Link, Size: result.Bytes, At: time.Now().Unix()})
	summary := fmt.Sprintf("uploaded \"%s\": %d files, %s", result.Name, result.Files, files.FormatSize(result.Bytes))
	if target.secret == nil {
		m.notice(target, summary+". Get the link with /link "+result.Name+", or open a chat and /share "+result.Name)
		return
	}
	m.notice(target, summary)
	m.reportShare(target, result.Name, result.Bytes)
	if err := m.send(target, payload{Kind: "file", Handle: m.session.handle, Name: result.Name, Link: result.Link, Size: result.Bytes}); err != nil {
		m.notice(target, "link not sent: "+err.Error()+" (use /share "+result.Name+" later)")
	}
}

func (m *Model) findGroupState(target *groupState) int {
	for index, g := range m.groups {
		if g == target {
			return index
		}
	}
	return -1
}

// startDownload handles /download SHORTNAME|LINK [DIR].
func (m *Model) startDownload(current *groupState, argument string) tea.Cmd {
	arguments := files.SplitArguments(argument)
	if len(arguments) == 0 || len(arguments) > 2 {
		m.notice(current, "usage: /download SHORTNAME|LINK [TARGET_DIR]  (default: current directory)")
		return nil
	}
	if m.busy {
		m.notice(current, "another transfer is still running")
		return nil
	}
	target := "."
	if len(arguments) == 2 {
		target = files.ExpandHome(arguments[1])
	}
	name, link := arguments[0], arguments[0]
	if record, found := m.vault.Files[name]; found {
		link = record.Link
	} else if !strings.Contains(name, "://") {
		m.notice(current, "no file named "+name+", see /files")
		return nil
	} else {
		name = "file"
	}
	address, _, err := files.ParseLink(link)
	if err != nil {
		m.notice(current, err.Error())
		return nil
	}
	m.busy, m.transfer = true, "downloading "+name
	m.notice(current, "downloading "+name+" from "+strings.SplitN(strings.TrimPrefix(strings.TrimPrefix(address, "http://"), "https://"), "/", 2)[0]+" to "+target+"...")
	return func() tea.Msg { return files.Download(m.server, link, target, name) }
}

// ---- connect screen ----

func (m *Model) updateConnect(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.String() != "enter" {
		var command tea.Cmd
		m.connectInput, command = m.connectInput.Update(key)
		return m, command
	}

	text := strings.TrimSpace(m.connectInput.Value())
	m.connectInput.Reset()
	name, argument, _ := strings.Cut(text, " ")
	switch name {
	case "/connect":
		address, err := ParseServerURL(argument)
		if err != nil {
			m.connectError = err.Error()
		} else {
			m.connectTo(address)
		}
	case "/quit", "/exit":
		return m, tea.Quit
	default:
		m.connectError = "type /connect followed by the server's address"
	}
	return m, nil
}

func (m *Model) connectView() string {
	status := ui.Dim.Render("Ctrl+C to quit")
	switch {
	case m.connectError != "":
		status = ui.Error.Render(m.connectError)
	case m.server != nil:
		status = ui.Dim.Render("connecting to " + m.server.URL() + "...")
	}
	lines := []string{
		ui.Title.Render("Connect to a chat server"),
		"",
		"Type /connect followed by the server's address, for example:",
		ui.Dim.Render("  /connect pipechat.kibandaskimart.com"),
		ui.Dim.Render("  /connect 192.168.1.5:7777"),
		"",
		m.connectInput.View(),
		"",
		status,
	}
	box := ui.AuthBox.Render(strings.Join(lines, "\n"))
	return lipgloss.Place(max(m.width, 1), max(m.height, 1), lipgloss.Center, lipgloss.Center, box)
}

// shortError drops the "Get http://..." wrapping from network errors.
func shortError(err error) string {
	var failure *url.Error
	if errors.As(err, &failure) {
		return failure.Err.Error()
	}
	return err.Error()
}
