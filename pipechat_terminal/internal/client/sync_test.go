package client

import (
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
CREATE TABLE chat_groups (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL, owner_id BIGINT NOT NULL REFERENCES users(id), gid TEXT UNIQUE);
CREATE TABLE chat_group_members (id BIGSERIAL PRIMARY KEY, group_id BIGINT NOT NULL REFERENCES chat_groups(id), user_id BIGINT NOT NULL REFERENCES users(id), delivered BOOLEAN NOT NULL DEFAULT false, UNIQUE (group_id, user_id));
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

func TestWebsiteGroupReachesTheTerminal(t *testing.T) {
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
	handlers.Pool, handlers.Config = pool, &config.Config{FilesDir: t.TempDir(), MaxUploadBytes: 1000, FileDays: 7}
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

	alice := newTerminal(t, server.URL, "alice@example.com")
	bob := newTerminal(t, server.URL, "bob@example.com")

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

	// Bob accepts the invitation on the website; nothing happens until alice's terminal syncs.
	pool.Exec(`INSERT INTO chat_group_members (group_id, user_id) VALUES ($1, $2)`, group, ids["bob"])
	bob.sync()
	if len(bob.m.groups) != 1 {
		t.Fatal("bob got a group without a key")
	}
	alice.sync()
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
}
