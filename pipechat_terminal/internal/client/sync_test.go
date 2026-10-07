package client

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"os"
	"os/exec"
	"path/filepath"
	"pipechat/internal/client/music"
	"pipechat/internal/client/update"
	"regexp"
	"runtime"
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
CREATE TABLE users (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL DEFAULT '', handle TEXT UNIQUE, bio TEXT, remember_token TEXT, created_at TIMESTAMP DEFAULT now(), updated_at TIMESTAMP, email TEXT NOT NULL UNIQUE, password TEXT NOT NULL);
CREATE TABLE chat_groups (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL, owner_id BIGINT NOT NULL REFERENCES users(id), gid TEXT UNIQUE, escrow BYTEA);
CREATE TABLE chat_group_members (id BIGSERIAL PRIMARY KEY, group_id BIGINT NOT NULL REFERENCES chat_groups(id), user_id BIGINT NOT NULL REFERENCES users(id), delivered BOOLEAN NOT NULL DEFAULT false, created_at TIMESTAMP, updated_at TIMESTAMP, UNIQUE (group_id, user_id));
CREATE TABLE chat_group_invite_codes (id BIGSERIAL PRIMARY KEY, group_id BIGINT NOT NULL REFERENCES chat_groups(id), code TEXT NOT NULL UNIQUE, uses INT NOT NULL DEFAULT 0, max_uses INT NOT NULL DEFAULT 25, expires_at TIMESTAMP NOT NULL, created_at TIMESTAMP, updated_at TIMESTAMP);
CREATE TABLE chat_group_invites (id BIGSERIAL PRIMARY KEY, group_id BIGINT NOT NULL REFERENCES chat_groups(id), inviter_id BIGINT NOT NULL REFERENCES users(id), invitee_id BIGINT NOT NULL REFERENCES users(id), status TEXT NOT NULL DEFAULT 'pending');
CREATE TABLE tracks (id BIGSERIAL PRIMARY KEY, title TEXT NOT NULL, artist TEXT, path TEXT NOT NULL, extension TEXT NOT NULL, size BIGINT NOT NULL, md5 TEXT, uploaded_by BIGINT REFERENCES users(id));
CREATE TABLE chat_group_files (id BIGSERIAL PRIMARY KEY, group_id BIGINT NOT NULL REFERENCES chat_groups(id), user_id BIGINT REFERENCES users(id), name TEXT NOT NULL, size BIGINT NOT NULL, md5 TEXT, expires_at TIMESTAMP, created_at TIMESTAMP);
CREATE TABLE binaries (id BIGSERIAL PRIMARY KEY, os TEXT NOT NULL, arch TEXT NOT NULL, version TEXT NOT NULL, filename TEXT NOT NULL, path TEXT NOT NULL, size BIGINT NOT NULL, md5 TEXT NOT NULL, sha256 TEXT NOT NULL);
CREATE TABLE password_reset_tokens (email TEXT PRIMARY KEY, token TEXT NOT NULL, created_at TIMESTAMP);
CREATE TABLE sessions (id TEXT PRIMARY KEY, user_id BIGINT);
CREATE TABLE chat_group_grants (group_id BIGINT NOT NULL REFERENCES chat_groups(id), user_id BIGINT NOT NULL REFERENCES users(id), from_user_id BIGINT NOT NULL REFERENCES users(id), sealed BYTEA NOT NULL, created_at TIMESTAMP, PRIMARY KEY (group_id, user_id));`

// terminal is a client Model wired to a real server, with its events pumped by hand.
type terminal struct {
	t        *testing.T
	m        *Model
	messages chan tea.Msg
}

func newTerminal(t *testing.T, url string, email string) *terminal {
	store, _ := LoadStore(filepath.Join(t.TempDir(), "client.json"))
	return newTerminalWithStore(t, url, email, store)
}

// newTerminalWithStore is a terminal on a computer whose settings file is already there, as when
// someone logs in again, or a second person uses the same machine.
func newTerminalWithStore(t *testing.T, url string, email string, store *Store) *terminal {
	m := NewModel(store, "")
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
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
		case <-time.After(25 * time.Millisecond):
			// look at the conditions again: some, like a file written by a player, change without a message
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

// startTestServer runs the chat server on a fresh schema of the test database, with the website's tables.
func startTestServer(t *testing.T, escrow bool) (url string, pool *sql.DB) {
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
	pool, err = database.Connect(context.Background(), base+separator+"search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close() })
	if _, err := pool.Exec(websiteSchema); err != nil {
		t.Fatal(err)
	}
	settings := &config.Config{FilesDir: t.TempDir(), MaxUploadBytes: 1000, FileDays: 7, StorageDir: t.TempDir()}
	if escrow {
		settings.EscrowKey = bytes.Repeat([]byte{7}, 32)
	}
	handlers.Pool, handlers.Config = pool, settings
	server := httptest.NewServer(handlers.Routes())
	t.Cleanup(server.Close)
	return server.URL, pool
}

func websiteGroupFlow(t *testing.T, escrow bool) (alice, bob *terminal, pool *sql.DB) {
	serverURL, pool := startTestServer(t, escrow)

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

	alice = newTerminal(t, serverURL, "alice@example.com")
	bob = newTerminal(t, serverURL, "bob@example.com")

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
	return alice, bob, pool
}

// The creator removes a member; the member's terminal drops the group when it next tries to join.
func TestKickedMemberLosesTheGroup(t *testing.T) {
	alice, bob, _ := websiteGroupFlow(t, true)

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

func (term *terminal) said() string {
	var all []string
	for _, g := range term.m.groups[1:] {
		for _, line := range g.entries {
			all = append(all, line.text)
		}
	}
	return strings.Join(all, "\n")
}

// Messages sent while someone was away are there when they log in again, and what one person
// has read on a shared computer does not hide messages from the next person.
func TestMessagesSentWhileAwayArriveAtLogin(t *testing.T) {
	alice, bob, _ := websiteGroupFlow(t, true)
	url := alice.m.server.URL()
	bobsStore := bob.m.store

	// Bob closes the terminal. Alice keeps talking.
	bob.m.server.Stop()
	for _, body := range []string{"first while you were away", "second while you were away"} {
		if err := alice.m.send(alice.m.groups[1], payload{Kind: "msg", Handle: "alice", Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	// The test has no WebSocket, so give the server a moment to store them.
	time.Sleep(time.Second)

	// Bob logs in again on the same computer: he gets exactly what he missed.
	bobAgain := newTerminalWithStore(t, url, "bob@example.com", bobsStore)
	bobAgain.until("the missed messages", func() bool { return strings.Contains(bobAgain.said(), "second while you were away") })
	if !strings.Contains(bobAgain.said(), "first while you were away") {
		t.Fatal("only some of the missed messages arrived")
	}

	// Closing and reopening the terminal brings back what was already read, with nothing new from the server.
	bobAgain.m.server.Stop()
	bobThird := newTerminalWithStore(t, url, "bob@example.com", bobsStore)
	if !strings.Contains(bobThird.said(), "first while you were away") || !strings.Contains(bobThird.said(), "second while you were away") {
		t.Fatalf("earlier messages should come back from the saved history:\n%s", bobThird.said())
	}

	// Alice now uses Bob's computer. His settings must not make her miss anything.
	aliceThere := newTerminalWithStore(t, url, "alice@example.com", bobsStore)
	aliceThere.until("alice's whole history", func() bool { return strings.Contains(aliceThere.said(), "first while you were away") })
}

// musicSetup makes a library of three tracks and a stand-in player. It returns the terminal, and a function
// that reads the list of what the player was asked to play (one address or file per line).
func musicSetup(t *testing.T, playerName string, script string) (*terminal, func() []string, string) {
	serverURL, pool := startTestServer(t, false)
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct horse"), bcrypt.MinCost)
	var userID int64
	pool.QueryRow(`INSERT INTO users (handle, email, password) VALUES ('alice', 'alice@example.com', $1) RETURNING id`, string(hash)).Scan(&userID)

	if err := os.MkdirAll(filepath.Join(handlers.Config.StorageDir, "music"), 0o755); err != nil {
		t.Fatal(err)
	}
	for number := 1; number <= 3; number++ {
		song := bytes.Repeat([]byte{byte('a' + number)}, 100*number)
		name := fmt.Sprintf("music/%d.mp3", number)
		os.WriteFile(filepath.Join(handlers.Config.StorageDir, name), song, 0o644)
		sum := md5.Sum(song)
		pool.Exec(`INSERT INTO tracks (title, artist, path, extension, size, md5, uploaded_by) VALUES ($1, 'The Tests', $2, 'mp3', $3, $4, $5)`,
			fmt.Sprintf("Song %d", number), name, len(song), hex.EncodeToString(sum[:]), userID)
	}

	bin := t.TempDir()
	record := filepath.Join(t.TempDir(), "played.txt")
	fake := filepath.Join(bin, playerName)
	os.WriteFile(fake, []byte("#!/bin/sh\nfor last; do :; done\n"+strings.ReplaceAll(script, "RECORD", record)+"\n"), 0o755)
	previous := music.LookPath
	music.LookPath = func(name string) (string, error) {
		if name == playerName {
			return fake, nil
		}
		return "", os.ErrNotExist
	}
	t.Cleanup(func() { music.LookPath = previous })
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	played := func() []string {
		data, _ := os.ReadFile(record)
		return strings.Fields(string(data))
	}
	return newTerminal(t, serverURL, "alice@example.com"), played, record
}

func (term *terminal) run(command string) {
	term.m.input.SetValue(command)
	term.m.Update(tea.KeyMsg{Type: tea.KeyEnter})
}

// order is the track numbers in the addresses a player was given.
func order(played []string) string {
	var numbers []string
	for _, address := range played {
		name := address[strings.LastIndex(address, "/")+1:]
		numbers = append(numbers, strings.TrimSuffix(name, ".mp3"))
	}
	return strings.Join(numbers, " ")
}

func (term *terminal) waitPlays(played func() []string, count int) {
	term.t.Helper()
	term.until(fmt.Sprintf("%d songs to play", count), func() bool { return len(played()) >= count })
}

func TestStreamingPlaysTheLibraryInOrder(t *testing.T) {
	term, played, _ := musicSetup(t, "mpv", `echo "$last" >> RECORD; sleep 0.1`)
	term.run("/music")
	term.until("the library", func() bool { return term.m.showHelp && strings.Contains(term.m.View(), "Song 3") })
	term.press("esc")

	term.run("/play")
	term.until("the queue to finish", func() bool { return strings.Contains(entriesText(term.m.groups[0]), "queue is finished") })
	if got := order(played()); got != "1 2 3" {
		t.Fatalf("played %q, want %q", got, "1 2 3")
	}

	// The player was given an address on this computer that streams the real bytes, and seeking works.
	address := played()[1]
	if !strings.HasPrefix(address, "http://127.0.0.1:") {
		t.Fatalf("expected a local stream address, got %q", address)
	}
	response, err := http.Get(address)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if !bytes.Equal(body, bytes.Repeat([]byte{'c'}, 200)) {
		t.Fatalf("the stream carried %d wrong bytes", len(body))
	}
	request, _ := http.NewRequest("GET", address, nil)
	request.Header.Set("Range", "bytes=10-19")
	ranged, _ := http.DefaultClient.Do(request)
	part, _ := io.ReadAll(ranged.Body)
	ranged.Body.Close()
	if ranged.StatusCode != http.StatusPartialContent || len(part) != 10 {
		t.Fatalf("range: %d, %d bytes", ranged.StatusCode, len(part))
	}
	// Nobody else on this computer can use the stream without its secret.
	other := address[:strings.LastIndex(address[:strings.LastIndex(address, "/")], "/")] + "/wrong/2.mp3"
	if guess, err := http.Get(other); err != nil || guess.StatusCode != http.StatusNotFound {
		t.Fatalf("a wrong secret must not work: %v %v", guess, err)
	}
}

func TestShuffleAndStartingPoint(t *testing.T) {
	previous := music.Shuffle
	music.Shuffle = func(tracks []music.Track) { // a fixed "random" order: backwards
		for i, j := 0, len(tracks)-1; i < j; i, j = i+1, j-1 {
			tracks[i], tracks[j] = tracks[j], tracks[i]
		}
	}
	defer func() { music.Shuffle = previous }()

	term, played, _ := musicSetup(t, "mpv", `echo "$last" >> RECORD; sleep 0.1`)
	term.run("/shuffle on")
	term.run("/play 2") // starts with 2, then the rest shuffled
	term.until("the queue to finish", func() bool { return strings.Contains(entriesText(term.m.groups[0]), "queue is finished") })
	if got := order(played()); got != "2 3 1" {
		t.Fatalf("shuffled from 2: %q", got)
	}
}

func TestRepeatOneAndAll(t *testing.T) {
	term, played, _ := musicSetup(t, "mpv", `echo "$last" >> RECORD; sleep 0.1`)

	term.run("/repeat one")
	term.run("/play 2")
	term.waitPlays(played, 3)
	term.run("/stop")
	if got := order(played()[:3]); got != "2 2 2" {
		t.Fatalf("repeat one: %q", got)
	}

	// A fresh run, repeating the whole library.
	term.run("/repeat all")
	before := len(played())
	term.run("/play")
	term.waitPlays(played, before+5)
	term.run("/stop")
	if got := order(played()[before : before+5]); got != "1 2 3 1 2" {
		t.Fatalf("repeat all: %q", got)
	}
	if term.m.playing != nil || len(term.m.list.Queue) != 0 {
		t.Fatal("/stop should end the queue")
	}
	if !strings.Contains(term.m.list.Modes(), "repeat all") {
		t.Fatalf("mode text: %q", term.m.list.Modes())
	}
}

func TestNextSkipsToTheFollowingSong(t *testing.T) {
	term, played, _ := musicSetup(t, "mpv", `echo "$last" >> RECORD; sleep 5`)
	term.run("/play")
	term.waitPlays(played, 1)
	term.run("/next")
	term.waitPlays(played, 2)
	term.run("/stop")
	if got := order(played()[:2]); got != "1 2" {
		t.Fatalf("after /next: %q", got)
	}
}

func TestAPlayerThatFailsDoesNotLoopForever(t *testing.T) {
	term, played, _ := musicSetup(t, "mpv", `echo "$last" >> RECORD; exit 1`)
	term.run("/repeat all")
	term.run("/play")
	term.until("the error", func() bool { return strings.Contains(entriesText(term.m.groups[0]), "stopped with an error") })
	if len(played()) != 1 || len(term.m.list.Queue) != 0 {
		t.Fatalf("should give up after one failure: %d plays", len(played()))
	}
}

// A player that cannot stream (afplay) is given a downloaded file instead.
func TestPlayersThatCannotStreamGetAFile(t *testing.T) {
	term, played, record := musicSetup(t, "afplay", `case "$last" in http*) echo "stream" >> RECORD;; *) cat "$last" > RECORD.data; echo "$last" >> RECORD;; esac`)
	term.run("/play 3")
	term.until("the queue to finish", func() bool { return strings.Contains(entriesText(term.m.groups[0]), "queue is finished") })
	data, err := os.ReadFile(record + ".data")
	if len(played()) == 0 || strings.Contains(played()[0], "http") || err != nil || len(data) == 0 {
		t.Fatalf("expected a downloaded file to be played: %v %v", played(), err)
	}
}

func entriesText(g *groupState) string {
	var all []string
	for _, line := range g.entries {
		all = append(all, line.text)
	}
	return strings.Join(all, "\n")
}

func TestWhoisOpensAPageWithTheBio(t *testing.T) {
	serverURL, pool := startTestServer(t, false)
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct horse"), bcrypt.MinCost)
	for _, handle := range []string{"alice", "bobby"} {
		pool.Exec(`INSERT INTO users (handle, name, email, password) VALUES ($1, $2, $3, $4)`, handle, strings.ToUpper(handle[:1])+handle[1:], handle+"@example.com", string(hash))
	}
	alice := newTerminal(t, serverURL, "alice@example.com")
	bobby := newTerminal(t, serverURL, "bobby@example.com")

	alice.run("/bio Writes Go and plays the cello")
	alice.until("the bio to be saved", func() bool { return strings.Contains(entriesText(alice.m.groups[0]), "your bio is saved") })

	// Bobby looks Alice up: a page of its own opens, and the chat is out of the way.
	bobby.run("/whois @alice")
	bobby.until("the page", func() bool { return bobby.m.showHelp })
	view := bobby.m.View()
	for _, want := range []string{"@alice", "Alice", "Writes Go and plays the cello", "Member since", "Key fingerprint", "not saved yet", "/dm alice"} {
		if !strings.Contains(view, want) {
			t.Errorf("the page lacks %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Welcome to Pipechat") {
		t.Error("the page should replace the chat")
	}
	bobby.m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if bobby.m.showHelp {
		t.Error("Esc should close the page")
	}

	// A person's own page offers to change the bio; one with no bio says so.
	bobby.run("/me")
	bobby.until("his own page", func() bool { return bobby.m.showHelp })
	if view := bobby.m.View(); !strings.Contains(view, "/bio TEXT") || !strings.Contains(view, "whois @bobby") {
		t.Errorf("own page:\n%s", view)
	}
	bobby.m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	bobby.run("/whois nobody")
	bobby.until("an error", func() bool { return strings.Contains(entriesText(bobby.m.groups[0]), "no user @nobody") })
	if bobby.m.showHelp {
		t.Error("an unknown user should not open a page")
	}

	alice.run("/bio clear")
	alice.until("it to be cleared", func() bool { return strings.Contains(entriesText(alice.m.groups[0]), "bio is cleared") })
}

func twoUsers(t *testing.T) (alice, bobby *terminal) {
	serverURL, pool := startTestServer(t, false)
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct horse"), bcrypt.MinCost)
	for _, handle := range []string{"alice", "bobby"} {
		pool.Exec(`INSERT INTO users (handle, name, email, password) VALUES ($1, $2, $3, $4)`, handle, handle, handle+"@example.com", string(hash))
	}
	return newTerminal(t, serverURL, "alice@example.com"), newTerminal(t, serverURL, "bobby@example.com")
}

func (term *terminal) heard(text string) func() bool {
	return func() bool {
		return strings.Contains(entriesText(term.m.groups[0]), text) || strings.Contains(term.said(), text)
	}
}

func TestAnyoneCanJoinAPublicGroupByName(t *testing.T) {
	alice, bobby := twoUsers(t)

	alice.run("/create crew-chat -open")
	alice.until("the group", func() bool { return len(alice.m.groups) == 2 && !alice.m.groups[1].adding })
	if !strings.Contains(entriesText(alice.m.groups[1]), "anyone can join it with /join crew-chat") {
		t.Fatalf("alice should be told it is public:\n%s", entriesText(alice.m.groups[1]))
	}

	// Another person sees it listed and joins by typing its name.
	time.Sleep(500 * time.Millisecond) // alice's terminal lists the new group right after the server has it
	bobby.run("/public")
	bobby.until("the list", bobby.heard("crew-chat"))
	bobby.run("/join CREW-chat") // names do not care about capital letters
	bobby.until("the group to arrive", func() bool { return len(bobby.m.groups) == 2 && !bobby.m.groups[1].adding })
	if bobby.m.groups[1].secret.Code() != alice.m.groups[1].secret.Code() {
		t.Fatal("bobby has a different key")
	}

	// A public name is unique, and a name that is not public gets a helpful answer.
	bobby.run("/create Crew-Chat -open")
	bobby.until("the refusal", bobby.heard("already exists"))
	if len(bobby.m.groups) != 2 {
		t.Fatalf("no second group should be made: %d chats", len(bobby.m.groups))
	}
	bobby.run("/join nobody-here")
	bobby.until("not found", bobby.heard("no public group called nobody-here"))
	bobby.run("/create ab -open")
	bobby.until("name rules", bobby.heard("3-24 characters"))

	// The creator can make it private again: nobody new can join by name.
	alice.run("/unlist crew-chat")
	alice.until("unlisted", alice.heard("no longer listed"))
	bobby.run("/leave")
	bobby.run("/join crew-chat")
	bobby.until("no longer public", func() bool {
		return strings.Count(entriesText(bobby.m.groups[0]), "no public group called crew-chat") >= 1
	})
	if len(bobby.m.groups) != 1 {
		t.Fatal("bobby should not get back in")
	}
}

func TestClosingAChatKeepsItOutOfTheSidebarUntilReopened(t *testing.T) {
	alice, _ := twoUsers(t)
	alice.run("/create notes")
	alice.until("the group", func() bool { return len(alice.m.groups) == 2 && !alice.m.groups[1].adding })
	if err := alice.m.send(alice.m.groups[1], payload{Kind: "msg", Handle: "alice", Body: "remember the milk"}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	alice.m.server.report(SocketEvent{}) // wake the loop

	alice.run("/close")
	if len(alice.m.groups) != 1 || len(alice.m.vault.Closed) != 1 {
		t.Fatalf("the chat should leave the sidebar: %d chats, %d closed", len(alice.m.groups), len(alice.m.vault.Closed))
	}
	alice.run("/closed")
	if !strings.Contains(entriesText(alice.m.groups[0]), "  notes") {
		t.Fatalf("/closed should list it:\n%s", entriesText(alice.m.groups[0]))
	}
	alice.run("/close") // on the welcome page
	if !strings.Contains(entriesText(alice.m.groups[0]), "welcome page") {
		t.Fatal("the welcome page cannot be closed")
	}

	// It stays closed after logging in again, and /reopen brings it back, with its saved messages.
	again := newTerminalWithStore(t, alice.m.server.URL(), "alice@example.com", alice.m.store)
	if len(again.m.groups) != 1 || len(again.m.vault.Closed) != 1 {
		t.Fatalf("closed chats must stay closed: %d chats", len(again.m.groups))
	}
	again.run("/reopen NOTES")
	again.until("the chat to come back", func() bool { return len(again.m.groups) == 2 && !again.m.groups[1].adding })
	if len(again.m.vault.Closed) != 0 {
		t.Fatal("it should leave the closed list")
	}
	again.run("/reopen nothing")
	again.until("not found", again.heard("no closed chat called nothing"))
}

// Previewing a file and saying yes uploads it, and the group is told.
func TestConfirmingThePreviewUploads(t *testing.T) {
	alice, _ := twoUsers(t)
	alice.run("/create share-here")
	alice.until("the group", func() bool { return len(alice.m.groups) == 2 && !alice.m.groups[1].adding })

	file := filepath.Join(t.TempDir(), "hello.txt")
	os.WriteFile(file, []byte("hello from alice\n"), 0o644)

	alice.m.input.SetValue("/upload " + file)
	alice.m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
	if !alice.m.showHelp || alice.m.busy {
		t.Fatal("the preview should be open and nothing uploaded yet")
	}
	_, command := alice.m.updateChat(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if alice.m.showHelp || !alice.m.busy {
		t.Fatal("saying yes should close the page and start the upload")
	}
	var messages []tea.Msg
	var collect func(tea.Cmd)
	collect = func(c tea.Cmd) {
		if c == nil {
			return
		}
		switch message := c().(type) {
		case tea.BatchMsg:
			for _, inner := range message {
				collect(inner)
			}
		default:
			messages = append(messages, message)
		}
	}
	collect(command)
	for _, message := range messages {
		alice.m.Update(message)
	}
	if !strings.Contains(entriesText(alice.m.groups[1]), `uploaded "hello.txt"`) {
		t.Fatalf("the upload should finish:\n%s", entriesText(alice.m.groups[1]))
	}
}

// ---- updating the program ----

// aProgramFor is bytes that pass for a program on the system running the test.
func aProgramFor(payload string) []byte {
	switch runtime.GOOS {
	case "windows":
		return append([]byte("MZ\x90\x00"), payload...)
	case "darwin":
		return append([]byte("\xcf\xfa\xed\xfe"), payload...)
	}
	return append([]byte("\x7fELF"), payload...)
}

// publishRelease puts a new client on the server the way the website's upload page does.
func publishRelease(t *testing.T, pool *sql.DB, version string, program []byte, checksumOverride string) {
	osName, arch, ok := update.Platform()
	if !ok {
		t.Skip("no downloads for this system")
	}
	relative := "downloads/pipechat-" + version
	os.MkdirAll(filepath.Join(handlers.Config.StorageDir, "downloads"), 0o755)
	os.WriteFile(filepath.Join(handlers.Config.StorageDir, relative), program, 0o755)
	sum := sha256.Sum256(program)
	hash := hex.EncodeToString(sum[:])
	if checksumOverride != "" {
		hash = checksumOverride
	}
	if _, err := pool.Exec(`INSERT INTO binaries (os, arch, version, filename, path, size, md5, sha256) VALUES ($1, $2, $3, $4, $5, $6, 'x', $7)`,
		osName, arch, version, "pipechat-"+version, relative, len(program), hash); err != nil {
		t.Fatal(err)
	}
}

// drive runs what the screen asked to be done, and gives the results back to it, until nothing is left. It
// returns whether the screen asked to quit.
func (term *terminal) drive(command tea.Cmd) (quit bool) {
	term.t.Helper()
	var run func(tea.Cmd)
	run = func(c tea.Cmd) {
		if c == nil {
			return
		}
		switch message := c().(type) {
		case nil:
		case tea.BatchMsg:
			for _, inner := range message {
				run(inner)
			}
		case tea.QuitMsg:
			quit = true
		default:
			_, next := term.m.Update(message)
			run(next)
		}
	}
	run(command)
	return quit
}

func (term *terminal) press(key string) tea.Cmd {
	var message tea.KeyMsg
	switch key {
	case "esc":
		message = tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		message = tea.KeyMsg{Type: tea.KeyEnter}
	default:
		message = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	_, command := term.m.Update(message)
	return command
}

// updateSetup runs a server with one user, makes this program claim to be version running, and points the
// updater at a stand-in file rather than the test program itself. It returns the server, its database and
// the stand-in's path.
func updateSetup(t *testing.T, running string) (url string, pool *sql.DB, exe string) {
	previous := update.Current
	update.Current = running
	t.Cleanup(func() { update.Current = previous })

	url, pool = startTestServer(t, false)
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct horse"), bcrypt.MinCost)
	pool.Exec(`INSERT INTO users (handle, name, email, password) VALUES ('alice', 'alice', 'alice@example.com', $1)`, string(hash))

	exe = filepath.Join(t.TempDir(), "pipechat")
	os.WriteFile(exe, aProgramFor("old"), 0o755)
	previousExecutable := executable
	executable = func() (string, error) { return exe, nil }
	t.Cleanup(func() { executable = previousExecutable })
	return url, pool, exe
}

func TestANewVersionIsOfferedAndInstalled(t *testing.T) {
	serverURL, pool, exe := updateSetup(t, "1.0.0")
	newProgram := aProgramFor(strings.Repeat("new version ", 3000))
	publishRelease(t, pool, "1.9.0", aProgramFor("an older build"), "")
	publishRelease(t, pool, "1.10.0", newProgram, "") // newest, though "1.9.0" sorts after it as text

	term := newTerminal(t, serverURL, "alice@example.com")
	term.until("the offer", func() bool { return term.m.pendingUpdate != nil && term.m.showHelp })
	view := term.m.View()
	for _, want := range []string{"A new version of Pipechat is ready", "v1.0.0", "v1.10.0", "update now", "skip this version", "Y update now"} {
		if !strings.Contains(view, want) {
			t.Errorf("the offer lacks %q:\n%s", want, view)
		}
	}

	// Yes: it downloads, installs, and asks the screen to close so the new program can start.
	quit := term.drive(term.press("y"))
	if !quit || term.m.RestartExe() != exe {
		t.Fatalf("after updating the screen should close and name the program: quit %v, restart %q\n%s", quit, term.m.RestartExe(), entriesText(term.m.groups[0]))
	}
	if got, _ := os.ReadFile(exe); !bytes.Equal(got, newProgram) {
		t.Fatalf("the program was not replaced (%d bytes)", len(got))
	}
	if term.m.busy {
		t.Fatal("the transfer should be over")
	}
}

func TestSayingNoOrSkipLeavesTheProgramAlone(t *testing.T) {
	serverURL, pool, exe := updateSetup(t, "1.0.0")
	publishRelease(t, pool, "1.1.0", aProgramFor("new"), "")

	term := newTerminal(t, serverURL, "alice@example.com")
	term.until("the offer", func() bool { return term.m.pendingUpdate != nil })

	term.drive(term.press("n"))
	if term.m.showHelp || term.m.pendingUpdate != nil || term.m.store.SkippedVersion != "" {
		t.Fatal("not now closes the page and remembers nothing")
	}
	if got, _ := os.ReadFile(exe); !bytes.Equal(got, aProgramFor("old")) {
		t.Fatal("the program must not change")
	}

	// /update brings the offer back.
	term.run("/update")
	term.until("the offer again", func() bool { return term.m.pendingUpdate != nil })

	// Skip: not offered again by itself, even on the next run, but /update still offers it.
	term.drive(term.press("s"))
	if term.m.store.SkippedVersion != "1.1.0" {
		t.Fatalf("skipped %q", term.m.store.SkippedVersion)
	}
	again := newTerminalWithStore(t, serverURL, "alice@example.com", term.m.store)
	time.Sleep(700 * time.Millisecond)
	again.until("a moment", func() bool { return time.Now().UnixMilli()%1 == 0 })
	if again.m.pendingUpdate != nil {
		t.Fatal("a skipped version must not be offered again by itself")
	}
	again.run("/update")
	again.until("the offer on request", func() bool { return again.m.pendingUpdate != nil })
}

func TestNoOfferWhenUpToDateOrDevelopmentBuild(t *testing.T) {
	for _, running := range []string{"1.1.0", "2.0.0", "dev"} {
		serverURL, pool, _ := updateSetup(t, running)
		publishRelease(t, pool, "1.1.0", aProgramFor("same"), "")
		term := newTerminal(t, serverURL, "alice@example.com")
		time.Sleep(600 * time.Millisecond)
		term.until("a moment", func() bool { return true })
		if term.m.pendingUpdate != nil || term.m.showHelp {
			t.Errorf("running %s: nothing newer exists, nothing should be offered", running)
		}
	}
	// Asking by hand says so in words.
	serverURL, pool, _ := updateSetup(t, "1.1.0")
	publishRelease(t, pool, "1.1.0", aProgramFor("same"), "")
	term := newTerminal(t, serverURL, "alice@example.com")
	term.run("/update")
	term.until("the answer", term.heard("you have the newest version (v1.1.0)"))
}

func TestABrokenDownloadNeverTouchesTheProgram(t *testing.T) {
	serverURL, pool, exe := updateSetup(t, "1.0.0")
	publishRelease(t, pool, "1.1.0", aProgramFor("new"), strings.Repeat("0", 64)) // the published checksum is wrong

	term := newTerminal(t, serverURL, "alice@example.com")
	term.until("the offer", func() bool { return term.m.pendingUpdate != nil })
	if quit := term.drive(term.press("y")); quit {
		t.Fatal("a failed update must not close the program")
	}
	if !strings.Contains(entriesText(term.m.groups[0]), "update failed") || !strings.Contains(entriesText(term.m.groups[0]), "checksum") {
		t.Fatalf("the failure should be explained:\n%s", entriesText(term.m.groups[0]))
	}
	if got, _ := os.ReadFile(exe); !bytes.Equal(got, aProgramFor("old")) {
		t.Fatal("the working program must be untouched")
	}
	if _, err := os.Stat(update.StagePath(exe)); err == nil {
		t.Fatal("the bad download must not stay on disk")
	}
	if term.m.busy {
		t.Fatal("the screen should be usable again")
	}
}

func TestSavingSongsToDisk(t *testing.T) {
	term, _, _ := musicSetup(t, "mpv", `exit 0`)
	dir := filepath.Join(t.TempDir(), "my music") // does not exist yet, and has a space in it

	term.run(`/save 2 "` + dir + `"`)
	term.until("the song to be saved", term.heard("saved 1 song(s)"))
	data, err := os.ReadFile(filepath.Join(dir, "The Tests - Song 2.mp3"))
	if err != nil || !bytes.Equal(data, bytes.Repeat([]byte{'c'}, 200)) {
		t.Fatalf("the file should be there, byte for byte: %v (%d bytes)", err, len(data))
	}
	if term.m.busy || term.m.progress != nil {
		t.Fatal("the transfer should be over")
	}

	// Saving again never overwrites what is there.
	term.run(`/save 2 "` + dir + `"`)
	term.until("the second copy", func() bool { _, err := os.Stat(filepath.Join(dir, "The Tests - Song 2 (2).mp3")); return err == nil })
	term.until("the report", func() bool { return strings.Count(entriesText(term.m.groups[0]), "saved 1 song(s)") == 2 })

	// The whole library.
	all := filepath.Join(t.TempDir(), "everything")
	term.run(`/save all ` + all)
	term.until("all of them", term.heard("saved 3 song(s)"))
	entries, _ := os.ReadDir(all)
	if len(entries) != 3 {
		t.Fatalf("saved %d files: %v", len(entries), entries)
	}

	// Mistakes get an answer and change nothing.
	for command, want := range map[string]string{
		"/save":           "usage: /save",
		"/save banana":    "nothing matches",
		"/save 1 a b":     "usage: /save",
		"/save 99 " + all: "nothing matches",
	} {
		term.run(command)
		term.until(command, term.heard(want))
	}
	if entries, _ := os.ReadDir(all); len(entries) != 3 {
		t.Fatal("a failed save must not leave files behind")
	}
}

func songHash(number int) string {
	sum := md5.Sum(bytes.Repeat([]byte{byte('a' + number)}, 100*number))
	return hex.EncodeToString(sum[:])
}

func TestSongsAreListedWithAHashAndNamedByNumberHashOrName(t *testing.T) {
	term, played, _ := musicSetup(t, "mpv", `echo "$last" >> RECORD; sleep 0.05`)

	term.run("/music")
	term.until("the library page", func() bool { return term.m.showHelp && term.m.pageTitle == "music" })
	page := term.m.View()
	for number := 1; number <= 3; number++ {
		for _, want := range []string{fmt.Sprintf("The Tests - Song %d.mp3", number), songHash(number)[:8]} {
			if !strings.Contains(page, want) {
				t.Errorf("the library page lacks %q:\n%s", want, page)
			}
		}
	}
	for _, want := range []string{"♪ Music library", "3 songs", "HASH", "SONG", "SIZE", "ADDED BY", "@alice", "select text to copy"} {
		if !strings.Contains(page, want) {
			t.Errorf("the library page lacks %q:\n%s", want, page)
		}
	}
	term.press("esc")
	if term.m.showHelp {
		t.Fatal("Esc should close the library page")
	}

	dir := t.TempDir()
	// The same song, named four ways, is saved each time (as copies, never over one another).
	for index, name := range []string{"2", "#2", songHash(2)[:6], "song 2", "The Tests - Song 2"} {
		term.run(`/save "` + name + `" "` + dir + `"`) // names with spaces go in quotes
		term.until("copy "+name, func() bool {         // one transfer at a time: wait for each to finish
			entries, _ := os.ReadDir(dir)
			return len(entries) == index+1 && !term.m.busy
		})
	}

	// Playing by hash starts there.
	term.run("/play " + songHash(3)[:5])
	term.until("it to play", func() bool { return len(played()) >= 1 })
	if got := order(played()[:1]); got != "3" {
		t.Fatalf("/play by hash started with %q", got)
	}
	term.run("/stop")

	term.run("/play nothing-like-this")
	term.until("not found", term.heard("nothing matches"))
}

func TestSharedFilesAreListedWithHashesAndDownloadedByAnyOfThem(t *testing.T) {
	alice, _ := twoUsers(t)
	alice.run("/create share-here")
	alice.until("the group", func() bool { return len(alice.m.groups) == 2 && !alice.m.groups[1].adding })

	content := bytes.Repeat([]byte("same bytes as md5sum sees\n"), 100)
	file := filepath.Join(t.TempDir(), "hello.txt")
	os.WriteFile(file, content, 0o644)
	sum := md5.Sum(content)
	want := hex.EncodeToString(sum[:])

	alice.m.input.SetValue("/upload " + file)
	alice.m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
	alice.drive(alice.press("y"))
	if got := alice.m.vault.Files["hello.txt"].Hash; got != want {
		t.Fatalf("the upload's hash is %q, md5sum says %q", got, want)
	}
	if !strings.Contains(entriesText(alice.m.groups[1]), "md5 "+want[:8]) {
		t.Fatalf("the upload should say its hash:\n%s", entriesText(alice.m.groups[1]))
	}

	alice.run("/files")
	alice.until("the files page", func() bool { return alice.m.showHelp && alice.m.pageTitle == "files" })
	page := alice.m.View()
	for _, wanted := range []string{"Shared files", "1 files", "HASH", "FILE", "hello.txt", want[:8], "you", "/download FILE"} {
		if !strings.Contains(page, wanted) {
			t.Fatalf("the files page lacks %q:\n%s", wanted, page)
		}
	}
	alice.press("esc")
	if alice.m.showHelp {
		t.Fatal("Esc closes the files page")
	}

	// By name, number, #number and the start of the hash: each fetches the file, checks its MD5, and says so.
	for _, name := range []string{"hello.txt", "1", "#1", want[:6]} {
		dir := t.TempDir()
		alice.m.input.SetValue("/download " + name + ` "` + dir + `"`)
		alice.drive(alice.pressEnter())
		if got, err := os.ReadFile(filepath.Join(dir, "hello.txt")); err != nil || !bytes.Equal(got, content) {
			t.Fatalf("/download %s: %v", name, err)
		}
		if !strings.Contains(entriesText(alice.m.groups[1]), "md5 "+want+" matches") {
			t.Fatalf("/download %s should say the MD5 matched:\n%s", name, entriesText(alice.m.groups[1]))
		}
		alice.m.groups[1].entries = nil
	}

	alice.run("/download nosuch")
	alice.until("an answer", alice.heard("nothing matches"))
	alice.run("/download 7")
	alice.until("an answer", alice.heard("nothing matches"))
}

// A share from another person carries its hash into the list; one that lies is refused at download.
func TestAHashFromAnotherPersonIsKeptAndChecked(t *testing.T) {
	alice, _ := twoUsers(t)
	alice.run("/create share-here")
	alice.until("the group", func() bool { return len(alice.m.groups) == 2 && !alice.m.groups[1].adding })
	group := alice.m.groups[1]

	hash := strings.Repeat("ab", 16)
	message, _ := json.Marshal(payload{Kind: "file", Handle: "bobby", Name: "photo.jpg", Link: "http://x/api/files/0123456789abcdef0123456789abcdef#k", Size: 99, Hash: hash})
	sealed, _ := group.secret.Seal(message)
	alice.m.receive(Event{Type: "msg", GID: group.secret.GIDHex(), CT: sealed, Seq: 1, TS: time.Now().UnixMilli()})
	if got := alice.m.vault.Files["photo.jpg"].Hash; got != hash {
		t.Fatalf("the sender's hash should be kept, got %q", got)
	}
	if !strings.Contains(entriesText(group), "md5 abababab") {
		t.Fatalf("the message should show the hash:\n%s", entriesText(group))
	}

	// A made-up hash is not kept.
	for index, bad := range []string{"not-hex", strings.Repeat("z", 32), "abc"} {
		message, _ := json.Marshal(payload{Kind: "file", Handle: "bobby", Name: fmt.Sprintf("bad%d.jpg", index), Link: "http://x/api/files/0123456789abcdef0123456789abcdef#k", Size: 1, Hash: bad})
		sealed, _ := group.secret.Seal(message)
		alice.m.receive(Event{Type: "msg", GID: group.secret.GIDHex(), CT: sealed, Seq: uint64(10 + index), TS: time.Now().UnixMilli()})
		if got := alice.m.vault.Files[fmt.Sprintf("bad%d.jpg", index)].Hash; got != "" {
			t.Errorf("%q should not be accepted as a hash, kept %q", bad, got)
		}
	}
}

func (term *terminal) pressEnter() tea.Cmd { return term.press("enter") }

// A song streams the first time and leaves a copy; the next time it is played from that copy.
func TestASongIsPlayedFromItsCopyAfterTheFirstTime(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("the stand-in player needs curl to listen to the stream")
	}
	term, played, _ := musicSetup(t, "mpv", `curl -s "$last" -o /dev/null; echo "$last" >> RECORD`)

	term.run("/play 2")
	term.waitPlays(played, 1)
	if !strings.HasPrefix(played()[0], "http://127.0.0.1:") {
		t.Fatalf("the first play streams: %q", played()[0])
	}
	term.run("/stop")
	track := music.Track{ID: 2, Ext: "mp3", Size: 200}
	term.until("the copy", func() bool { _, ok := music.Cached(track); return ok })

	term.run("/play 2")
	term.waitPlays(played, 2)
	if second := played()[1]; strings.HasPrefix(second, "http") || !strings.HasSuffix(second, filepath.Join("music", "2.mp3")) {
		t.Fatalf("the second play should use the copy on disk, got %q", second)
	}
	term.run("/stop")
}

// Against a real server: the website address it names reaches the login screen before anyone logs in.
func TestTheServerNamesItsWebsiteBeforeLogin(t *testing.T) {
	serverURL, _ := startTestServer(t, false)
	handlers.Config.WebsiteURL = "https://accounts.example.net"
	store, _ := LoadStore(filepath.Join(t.TempDir(), "client.json"))
	m := NewModel(store, "")
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	term := &terminal{t: t, m: m, messages: make(chan tea.Msg, 100)}
	m.server = NewServer(serverURL)
	m.server.report = func(message tea.Msg) { term.messages <- message }
	t.Cleanup(m.server.Stop)
	m.Update(ServerUp{})
	term.until("the server's answer", func() bool { return m.website != "" })
	if m.website != "https://accounts.example.net" || !strings.Contains(m.authView(), "https://accounts.example.net/register") {
		t.Fatalf("website %q:\n%s", m.website, m.authView())
	}
}

// The creator renames a group on the website; every member's terminal shows the new name, and keeps it.
func TestARenamedGroupShowsItsNewNameInTerminals(t *testing.T) {
	alice, bob, pool := websiteGroupFlow(t, true)
	if bob.m.groups[1].name() != "crew" {
		t.Fatalf("the group starts as %q", bob.m.groups[1].name())
	}

	pool.Exec(`UPDATE chat_groups SET name = 'night shift'`)
	bob.sync()
	if got := bob.m.groups[1].name(); got != "night shift" {
		t.Fatalf("after a sync bob sees %q", got)
	}
	if !strings.Contains(bob.m.View(), "night shift") || !strings.Contains(entriesText(bob.m.groups[0]), "The group crew is now called night shift") {
		t.Fatalf("the sidebar and a note should say so:\n%s", bob.m.View())
	}
	// The invite code still holds the old name, which is only used when there is nothing newer.
	if bob.m.groups[1].secret.Name != "crew" || bob.m.vault.Titles[bob.m.groups[1].secret.GIDHex()] != "night shift" {
		t.Fatal("the new name is kept in the vault, apart from the code")
	}

	// It is still there after the terminal is closed and opened again.
	time.Sleep(300 * time.Millisecond) // the vault is saved to the server in the background
	again := newTerminalWithStore(t, bob.m.server.URL(), "bob@example.com", bob.m.store)
	if len(again.m.groups) < 2 || again.m.groups[1].name() != "night shift" {
		t.Fatalf("a new session should open the group under its new name, groups: %d", len(again.m.groups))
	}

	// Another rename, and renaming back to what the code says, both work.
	pool.Exec(`UPDATE chat_groups SET name = 'crew'`)
	alice.sync()
	if got := alice.m.groups[1].name(); got != "crew" || alice.m.groups[1].title != "" {
		t.Fatalf("back to the original: %q (title %q)", got, alice.m.groups[1].title)
	}
	if len(alice.m.vault.Titles) != 0 {
		t.Fatalf("no title is kept when the name matches the code: %v", alice.m.vault.Titles)
	}
}

// A group keeps its id when it is renamed, also while it is closed: it can still be reopened, by its new name.
func TestARenamedGroupKeepsItsIdAndAClosedOneIsFoundByTheNewName(t *testing.T) {
	_, bob, pool := websiteGroupFlow(t, true)
	gid := bob.m.groups[1].secret.GIDHex()

	bob.run("/groups")
	bob.until("the list", bob.heard("(id "+gid[:8]+")"))

	bob.run("/close")
	if len(bob.m.groups) != 1 || len(bob.m.vault.Closed) != 1 || bob.m.vault.Closed[0].Name != "crew" {
		t.Fatalf("closed: %d chats, %+v", len(bob.m.groups), bob.m.vault.Closed)
	}
	pool.Exec(`UPDATE chat_groups SET name = 'night shift'`)
	bob.sync()
	if bob.m.vault.Closed[0].Name != "night shift" {
		t.Fatalf("a closed group follows its new name: %q", bob.m.vault.Closed[0].Name)
	}

	bob.run("/reopen crew")
	bob.until("the old name no longer matches", bob.heard("no closed chat called crew"))
	bob.run(`/reopen "night shift"`)
	bob.until("the group to come back", func() bool { return len(bob.m.groups) == 2 && !bob.m.groups[1].adding })
	reopened := bob.m.groups[1]
	if reopened.name() != "night shift" || reopened.secret.GIDHex() != gid {
		t.Fatalf("reopened as %q with id %s, wanted the same id %s", reopened.name(), reopened.secret.GIDHex(), gid)
	}
}

// ---- the screens before logging in ----

// anonymousTerminal is a terminal that has connected to a server but not logged in.
func anonymousTerminal(t *testing.T, serverURL string) *terminal {
	store, _ := LoadStore(filepath.Join(t.TempDir(), "client.json"))
	m := NewModel(store, "")
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	term := &terminal{t: t, m: m, messages: make(chan tea.Msg, 100)}
	m.server = NewServer(serverURL)
	m.server.report = func(message tea.Msg) { term.messages <- message }
	m.serverReached, m.online = true, true
	t.Cleanup(m.server.Stop)
	return term
}

// fill types values into the fields of the screen that is showing, in order, and presses Enter after each.
func (term *terminal) fill(values ...string) {
	for _, value := range values {
		field := term.m.fields()[term.m.focus]
		field.SetValue(value)
		term.m.updateAuth(tea.KeyMsg{Type: tea.KeyEnter})
	}
}

func (term *terminal) ctrl(key tea.KeyType) { term.m.updateAuth(tea.KeyMsg{Type: key}) }

func TestMakingAnAccountFromTheTerminalAndLoggingStraightIn(t *testing.T) {
	serverURL, pool := startTestServer(t, false)
	term := anonymousTerminal(t, serverURL)

	term.ctrl(tea.KeyCtrlR)
	view := term.m.authView()
	for _, want := range []string{"Create your account", "Your name", "Handle (what others see", "Email", "Password (8 or more", "Repeat the password", "Esc back to log in"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the register screen lacks %q:\n%s", want, view)
		}
	}

	// Mistakes are told at once, without asking the server.
	for _, c := range []struct {
		values []string
		says   string
	}{
		{[]string{"", "ann_e", "ann@example.com", "correct horse", "correct horse"}, "enter your name"},
		{[]string{"Ann", "A", "ann@example.com", "correct horse", "correct horse"}, "3-20 characters"},
		{[]string{"Ann", "ann_e", "not-an-email", "correct horse", "correct horse"}, "email address"},
		{[]string{"Ann", "ann_e", "ann@example.com", "short", "short"}, "at least 8"},
		{[]string{"Ann", "ann_e", "ann@example.com", "correct horse", "correct hors"}, "not the same"},
	} {
		term.m.goToAuth(modeRegister)
		term.m.emailInput.SetValue("")
		term.m.focusField(0)
		term.fill(c.values...)
		if !strings.Contains(term.m.authError, c.says) || term.m.authBusy {
			t.Fatalf("%v should say %q, got %q", c.values, c.says, term.m.authError)
		}
	}
	var users int
	pool.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users)
	if users != 0 {
		t.Fatal("nothing reached the server")
	}

	// The real thing: the account is made, and the same screen logs in with it.
	term.m.goToAuth(modeRegister)
	term.m.emailInput.SetValue("")
	term.m.focusField(0)
	term.fill("Ann Example", "ann_e", "Ann@Example.com", "correct horse", "correct horse")
	term.until("the new account to be logged in", func() bool { return term.m.loggedIn })
	if term.m.session.handle != "ann_e" {
		t.Fatalf("logged in as %q", term.m.session.handle)
	}
	var name, email string
	pool.QueryRow(`SELECT name, email FROM users WHERE handle = 'ann_e'`).Scan(&name, &email)
	if name != "Ann Example" || email != "ann@example.com" {
		t.Fatalf("the account: %q %q", name, email)
	}

	// A handle or email that is taken comes back as an answer on the same screen, and nothing is lost.
	again := anonymousTerminal(t, serverURL)
	again.ctrl(tea.KeyCtrlR)
	again.fill("Someone Else", "ann_e", "other@example.com", "correct horse", "correct horse")
	again.until("the server's answer", func() bool { return again.m.authError != "" })
	if !strings.Contains(again.m.authError, "handle is taken") || again.m.authMode != modeRegister || again.m.nameInput.Value() != "Someone Else" {
		t.Fatalf("a taken handle: %q (screen %q, name kept %q)", again.m.authError, again.m.authMode, again.m.nameInput.Value())
	}
	again.ctrl(tea.KeyEsc)
	if again.m.authMode != modeLogin || again.m.passwordInput.Value() != "" {
		t.Fatal("Esc goes back to the login screen, with no password left in it")
	}
}

func TestResettingAForgottenPasswordFromTheTerminal(t *testing.T) {
	serverURL, pool := startTestServer(t, false)
	hash, _ := bcrypt.GenerateFromPassword([]byte("old password"), bcrypt.MinCost)
	pool.Exec(`INSERT INTO users (handle, name, email, password) VALUES ('annie', 'Annie', 'annie@example.com', $1)`, string(hash))

	mails := make(chan string, 5)
	restore := handlers.SetMailSender(func(address string, a smtp.Auth, from string, to []string, message []byte) error {
		mails <- string(message)
		return nil
	})
	defer restore()
	handlers.Config.SMTPHost, handlers.Config.SMTPPort, handlers.Config.SMTPFrom = "smtp.test", "587", "Pipechat <noreply@example.com>"

	term := anonymousTerminal(t, serverURL)
	term.ctrl(tea.KeyCtrlF)
	if view := term.m.authView(); !strings.Contains(view, "Reset your password") || !strings.Contains(view, "I already have a code") {
		t.Fatalf("the forgot screen:\n%s", view)
	}
	term.fill("annie@example.com")

	// The code arrives by email, and the screen moves on to asking for it.
	var body string
	select {
	case body = <-mails:
	case <-time.After(5 * time.Second):
		t.Fatal("no email arrived")
	}
	term.until("the code screen", func() bool { return term.m.authMode == modeReset })
	code := regexp.MustCompile(`[A-Z2-9]{4}-[A-Z2-9]{4}-[A-Z2-9]{4}`).FindString(body)
	if code == "" || term.m.emailInput.Value() != "annie@example.com" || !strings.Contains(term.m.authView(), "a code is on its way") {
		t.Fatalf("code %q, email kept %q:\n%s", code, term.m.emailInput.Value(), term.m.authView())
	}
	if !strings.Contains(term.m.authView(), "start with new keys") {
		t.Fatal("the reset screen warns that the saved keys start over")
	}

	// A wrong code, and then the right one, with the new password typed twice.
	term.m.focusField(0)
	term.fill("annie@example.com", "AAAA-BBBB-CCCC", "brand new pass", "brand new pass")
	term.until("the refusal", func() bool { return term.m.authError != "" })
	if !strings.Contains(term.m.authError, "wrong or has expired") || term.m.authMode != modeReset {
		t.Fatalf("a wrong code: %q", term.m.authError)
	}
	term.m.focusField(1)
	term.fill(strings.ToLower(code), "brand new pass", "brand new pass")
	term.until("back at the login screen", func() bool {
		return term.m.authMode == modeLogin && strings.Contains(term.m.authNotice, "Password changed")
	})
	if term.m.emailInput.Value() != "annie@example.com" || term.m.passwordInput.Value() != "" {
		t.Fatal("the email is filled in for the login, and no password is kept")
	}

	// The new password logs in; the old one does not.
	term.m.focusField(0)
	term.fill("annie@example.com", "brand new pass")
	term.until("logged in", func() bool { return term.m.loggedIn })
	old := anonymousTerminal(t, serverURL)
	old.m.focusField(0)
	old.fill("annie@example.com", "old password")
	old.until("the refusal", func() bool { return old.m.authError != "" })
}

func TestTheAccountScreensKeepToTheirRoutes(t *testing.T) {
	serverURL, _ := startTestServer(t, false)
	term := anonymousTerminal(t, serverURL)

	term.ctrl(tea.KeyCtrlF)
	term.ctrl(tea.KeyCtrlT) // I already have a code
	if term.m.authMode != modeReset {
		t.Fatal("Ctrl+T on the forgot screen goes to the code screen")
	}
	term.ctrl(tea.KeyEsc)
	if term.m.authMode != modeLogin {
		t.Fatal("Esc goes back")
	}
	term.ctrl(tea.KeyCtrlT) // only on the forgot screen
	if term.m.authMode != modeLogin {
		t.Fatal("Ctrl+T does nothing on the login screen")
	}
	// A server that cannot send mail says so instead of pretending.
	term.ctrl(tea.KeyCtrlF)
	term.fill("someone@example.com")
	term.until("the server's answer", func() bool { return term.m.authError != "" })
	if !strings.Contains(term.m.authError, "cannot send email") || term.m.authMode != modeForgot {
		t.Fatalf("no mail set up: %q", term.m.authError)
	}
}
