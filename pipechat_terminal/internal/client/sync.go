package client

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"pipechat/internal/crypto"
)

// Groups made on the website. The website knows each group's name and members but
// never its key, so the terminals do the work, checking in every syncEvery:
//
//   - a group you own that has no key yet: create it here and report its id;
//   - a member waiting for a group you hold: seal its invite code to their public key;
//   - a key sealed to you: open it, join the group, and confirm;
//   - when the server holds group keys (it says so with a public key), give it a copy of
//     each group key you have, sealed so only the server can open it. New members then
//     get their key from the server at once, without another member being online.

const syncEvery = 15 * time.Second

type syncTick struct{}

type syncResult struct {
	sync SyncResult
	err  error
}

// syncTicks schedules the next check-in.
func syncTicks() tea.Cmd {
	return tea.Tick(syncEvery, func(time.Time) tea.Msg { return syncTick{} })
}

// syncAsync asks the server what the website wants done, unless a request is already out.
func (m *Model) syncAsync() {
	if !m.loggedIn || m.syncing {
		return
	}
	m.syncing = true
	m.server.enqueue(func() {
		result, err := m.server.Sync()
		m.server.report(syncResult{sync: result, err: err})
	})
}

func (m *Model) handleSync(result syncResult) {
	m.syncing = false
	if result.err != nil || !m.loggedIn || m.identity == nil {
		return // try again at the next tick
	}
	if m.syncBusy == nil {
		m.syncBusy = map[string]time.Time{}
	}
	// Work already under way is skipped, but tried again if it is still listed after a while.
	once := func(key string) bool {
		if started, busy := m.syncBusy[key]; busy && time.Since(started) < time.Minute {
			return false
		}
		m.syncBusy[key] = time.Now()
		return true
	}

	for _, item := range result.sync.Create {
		if !once(fmt.Sprintf("create/%d", item.ID)) {
			continue
		}
		secret, err := crypto.NewSecret(item.Name)
		if err != nil {
			continue
		}
		m.addGroup(secret, false)
		m.groups[len(m.groups)-1].webID = item.ID
	}

	for _, item := range result.sync.Grant {
		group := m.groupByGID(item.GID)
		if group == nil || !once(fmt.Sprintf("grant/%d/%s", item.Group, item.Handle)) {
			continue
		}
		if pinned, known := m.vault.Pins[item.Handle]; known && pinned != item.PubKey {
			m.notice(m.groups[0], "WARNING: the key of @"+item.Handle+" changed; not sharing "+group.secret.Name+" with them.")
			continue
		}
		public, err := crypto.ParsePublicKey(item.PubKey)
		if err != nil {
			continue
		}
		sealed, err := crypto.SealFor(m.identity, public, group.secret)
		if err != nil {
			continue
		}
		webGroup, handle := item.Group, item.Handle
		m.server.enqueue(func() { m.server.SyncGrant(webGroup, handle, sealed) })
	}

	if result.sync.ServerKey != "" {
		serverKey, err := crypto.ParsePublicKey(result.sync.ServerKey)
		for _, item := range result.sync.NeedEscrow {
			group := m.groupByGID(item.GID)
			if err != nil || group == nil || !once(fmt.Sprintf("escrow/%d", item.Group)) {
				continue
			}
			if sealed, err := crypto.SealAnon(serverKey, group.secret); err == nil {
				webGroup := item.Group
				m.server.enqueue(func() { m.server.SyncEscrow(webGroup, sealed) })
			}
		}
	}

	for _, item := range result.sync.Incoming {
		if !once(fmt.Sprintf("incoming/%d", item.Group)) {
			continue
		}
		if pinned, known := m.vault.Pins[item.From]; known && pinned != item.FromKey {
			m.notice(m.groups[0], "WARNING: the key of @"+item.From+" changed; refusing the group "+item.Name+" from them.")
			continue
		}
		sender, err := crypto.ParsePublicKey(item.FromKey)
		if err != nil {
			continue
		}
		secret, err := crypto.OpenFrom(m.identity, sender, item.Sealed)
		if err != nil {
			m.notice(m.groups[0], "could not open the group "+item.Name+" sent by @"+item.From)
			continue
		}
		m.vault.Pins[item.From] = item.FromKey
		if existing := m.groupByGID(secret.GIDHex()); existing != nil {
			webGroup := item.Group
			m.server.enqueue(func() { m.server.SyncGrantReceived(webGroup) })
			continue
		}
		m.addGroup(secret, true)
		added := m.groups[len(m.groups)-1]
		added.webID, added.webGrant = item.Group, true
		if item.FromKey == result.sync.ServerKey && result.sync.ServerKey != "" {
			m.notice(m.groups[0], "You were added to the group "+secret.Name+".")
		} else {
			m.notice(m.groups[0], "You were added to the group "+secret.Name+" by @"+item.From+".")
		}
	}
	m.refresh()
}

func (m *Model) groupByGID(gid string) *groupState {
	for _, g := range m.groups[1:] {
		if !g.direct && !g.adding && g.secret.GIDHex() == gid {
			return g
		}
	}
	return nil
}
