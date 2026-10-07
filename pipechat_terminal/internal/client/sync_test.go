package client

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"

	"pipechat/internal/config"
	"pipechat/internal/database"
	"pipechat/internal/handlers"
)

const websiteSchema = `
CREATE TABLE users (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL DEFAULT '', handle TEXT UNIQUE, email TEXT NOT NULL UNIQUE, password TEXT NOT NULL);
CREATE TABLE chat_groups (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL, owner_id BIGINT NOT NULL REFERENCES users(id), gid TEXT UNIQUE, escrow BYTEA);
CREATE TABLE chat_group_members (id BIGSERIAL PRIMARY KEY, group_id BIGINT NOT NULL REFERENCES chat_groups(id), user_id BIGINT NOT NULL REFERENCES users(id), delivered BOOLEAN NOT NULL DEFAULT false, UNIQUE (group_id, user_id));
CREATE TABLE chat_group_invites (id BIGSERIAL PRIMARY KEY, group_id BIGINT NOT NULL REFERENCES chat_groups(id), inviter_id BIGINT NOT NULL REFERENCES users(id), invitee_id BIGINT NOT NULL REFERENCES users(id), status TEXT NOT NULL DEFAULT 'pending');
CREATE TABLE chat_group_grants (group_id BIGINT NOT NULL REFERENCES chat_groups(id), user_id BIGINT NOT NULL REFERENCES users(id), from_user_id BIGINT NOT NULL REFERENCES users(id), sealed BYTEA NOT NULL, created_at TIMESTAMP, PRIMARY KEY (group_id, user_id));`

// terminal is a client Model wired to a real server, with its events pumped by hand.
type terminal struct {
	t        *testing.T
	m        *Model
	messages chan tea.Msg
}

func newTerminal(t *testing.T, url string, email string) *terminal {
	store, _ := LoadStore(filepath.Join(t.TempDir(), "client.json"))
	m := NewModel(store, "")
	term := &terminal{t: t, m: m, messages: make(chan tea.Msg, 100)}
	m.server = NewServer(url)
	m.server.report = func(message tea.Msg) { term.messages <- message }
	m.serverReached, m.online = true, true
	t.Cleanup(m.server.Stop)

	m.emailInput.SetValue(email)
	m.passwordInput.SetValue("correct horse")
	m.submitAuth()
	term.until("login", func() bool { return m.loggedIn })
	return term
}

// until feeds the screen what the server reports until done() is true.
func (term *terminal) until(what string, done func() bool) {
	term.t.Helper()
	deadline := time.After(10 * time.Second)
	for !done() {
		select {
		case message := <-term.messages:
			term.m.Update(message)
		case <-deadline:
			term.t.Fatalf("timed out waiting for %s", what)
		}
	}
}

func (term *terminal) sync() {
	term.m.syncAsync()
	term.until("sync", func() bool { return !term.m.syncing })
}

// Without escrow the key passes from member to member, so the inviter's terminal must be online.
func TestWebsiteGroupReachesTheTerminal(t *testing.T) { websiteGroupFlow(t, false) }

// With escrow the server hands the key over, so the new member gets in with nobody else online.
func TestWebsiteGroupWithEscrowNeedsNoOneOnline(t *testing.T) { websiteGroupFlow(t, true) }

func websiteGroupFlow(t *testing.T, escrow bool) (alice, bob *terminal) {
	base := os.Getenv("CHAT_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("set CHAT_TEST_DATABASE_URL to a PostgreSQL URL to run this test")
	}
	admin, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("client_test_%d", time.Now().UnixNano())
	admin.Exec("CREATE SCHEMA " + schema)
	t.Cleanup(func() { admin.Exec("DROP SCHEMA " + schema + " CASCADE"); admin.Close() })
	separator := "?"
	if strings.Contains(base, "?") {
		separator = "&"
	}
	pool, err := database.Connect(context.Background(), base+separator+"search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close() })
	if _, err := pool.Exec(websiteSchema); err != nil {
		t.Fatal(err)
	}
	settings := &config.Config{FilesDir: t.TempDir(), MaxUploadBytes: 1000, FileDays: 7}
	if escrow {
		settings.EscrowKey = bytes.Repeat([]byte{7}, 32)
	}
	handlers.Pool, handlers.Config = pool, settings
	server := httptest.NewServer(handlers.Routes())
	t.Cleanup(server.Close)

	hash, _ := bcrypt.GenerateFromPassword([]byte("correct horse"), bcrypt.MinCost)
	ids := map[string]int64{}
	for _, handle := range []string{"alice", "bob"} {
		var id int64
		pool.QueryRow(`INSERT INTO users (handle, email, password) VALUES ($1, $2, $3) RETURNING id`, handle, handle+"@example.com", string(hash)).Scan(&id)
		ids[handle] = id
	}
	var group int64
	pool.QueryRow(`INSERT INTO chat_groups (name, owner_id) VALUES ('crew', $1) RETURNING id`, ids["alice"]).Scan(&group)
	pool.Exec(`INSERT INTO chat_group_members (group_id, user_id) VALUES ($1, $2)`, group, ids["alice"])

	alice = newTerminal(t, server.URL, "alice@example.com")
	bob = newTerminal(t, server.URL, "bob@example.com")

	// Alice's terminal creates the group and tells the website its chat id.
	alice.sync()
	alice.until("the group to be created", func() bool { return len(alice.m.groups) == 2 && !alice.m.groups[1].adding })
	var gid sql.NullString
	for range 100 {
		pool.QueryRow(`SELECT gid FROM chat_groups WHERE id = $1`, group).Scan(&gid)
		if gid.Valid {
			break
		}
		alice.until("a moment", func() bool { time.Sleep(20 * time.Millisecond); return true })
	}
	if !gid.Valid || gid.String != alice.m.groups[1].secret.GIDHex() {
		t.Fatalf("website group id %v, terminal %s", gid, alice.m.groups[1].secret.GIDHex())
	}

	if escrow {
		// Alice's terminal gives the server a copy of the key on its next check-in.
		held := false
		for i := 0; i < 100 && !held; i++ {
			alice.m.syncBusy = nil
			alice.sync()
			pool.QueryRow(`SELECT escrow IS NOT NULL FROM chat_groups WHERE id = $1`, group).Scan(&held)
			alice.until("a moment", func() bool { time.Sleep(20 * time.Millisecond); return true })
		}
		if !held {
			t.Fatal("the server never got a copy of the key")
		}
	}

	// Bob accepts the invitation on the website; nothing happens until a terminal syncs.
	pool.Exec(`INSERT INTO chat_group_members (group_id, user_id) VALUES ($1, $2)`, group, ids["bob"])
	bob.sync()
	if len(bob.m.groups) != 1 {
		t.Fatal("bob got a group without a key")
	}
	if !escrow {
		alice.sync() // the key comes from alice's terminal
	}
	waitFor := func(what string, term *terminal, done func() bool) {
		term.until(what, func() bool {
			if !done() {
				time.Sleep(20 * time.Millisecond)
				term.m.syncBusy = nil
				term.sync()
			}
			return done()
		})
	}
	waitFor("the group to reach bob", bob, func() bool { return len(bob.m.groups) == 2 && !bob.m.groups[1].adding })
	if bob.m.groups[1].secret.Code() != alice.m.groups[1].secret.Code() {
		t.Fatal("bob has a different key")
	}
	delivered := false
	for i := 0; i < 100 && !delivered; i++ {
		pool.QueryRow(`SELECT delivered FROM chat_group_members WHERE group_id = $1 AND user_id = $2`, group, ids["bob"]).Scan(&delivered)
		bob.until("a moment", func() bool { time.Sleep(20 * time.Millisecond); return true })
	}
	if !delivered {
		t.Fatal("the website was not told bob has the group")
	}
	return alice, bob
}

// The creator removes a member; the member's terminal drops the group when it next tries to join.
func TestKickedMemberLosesTheGroup(t *testing.T) {
	alice, bob := websiteGroupFlow(t, true)

	alice.m.input.SetValue("/kick @bob")
	alice.m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	alice.until("the kick to be answered", func() bool {
		for _, line := range alice.m.groups[1].entries {
			if strings.Contains(line.text, "removed @bob") {
				return true
			}
		}
		return false
	})

	bob.m.joinGroupAsync(bob.m.groups[1])
	bob.until("bob to lose the group", func() bool { return len(bob.m.groups) == 1 })
	found := false
	for _, line := range bob.m.groups[0].entries {
		found = found || strings.Contains(line.text, "removed from the group crew")
	}
	if !found {
		t.Fatal("bob was not told why the group went")
	}

	// Logging in again does not bring the group back.
	bob2 := newTerminal(t, alice.m.server.URL(), "bob@example.com")
	if len(bob2.m.groups) != 1 {
		t.Fatalf("a removed member got the group back on login: %d groups", len(bob2.m.groups))
	}
}
