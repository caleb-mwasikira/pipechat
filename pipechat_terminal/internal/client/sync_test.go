package client

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"pipechat/internal/client/music"
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
CREATE TABLE users (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL DEFAULT '', handle TEXT UNIQUE, bio TEXT, created_at TIMESTAMP DEFAULT now(), updated_at TIMESTAMP, email TEXT NOT NULL UNIQUE, password TEXT NOT NULL);
CREATE TABLE chat_groups (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL, owner_id BIGINT NOT NULL REFERENCES users(id), gid TEXT UNIQUE, escrow BYTEA);
CREATE TABLE chat_group_members (id BIGSERIAL PRIMARY KEY, group_id BIGINT NOT NULL REFERENCES chat_groups(id), user_id BIGINT NOT NULL REFERENCES users(id), delivered BOOLEAN NOT NULL DEFAULT false, UNIQUE (group_id, user_id));
CREATE TABLE chat_group_invites (id BIGSERIAL PRIMARY KEY, group_id BIGINT NOT NULL REFERENCES chat_groups(id), inviter_id BIGINT NOT NULL REFERENCES users(id), invitee_id BIGINT NOT NULL REFERENCES users(id), status TEXT NOT NULL DEFAULT 'pending');
CREATE TABLE tracks (id BIGSERIAL PRIMARY KEY, title TEXT NOT NULL, artist TEXT, path TEXT NOT NULL, extension TEXT NOT NULL, size BIGINT NOT NULL, uploaded_by BIGINT REFERENCES users(id));
CREATE TABLE chat_group_files (id BIGSERIAL PRIMARY KEY, group_id BIGINT NOT NULL REFERENCES chat_groups(id), user_id BIGINT REFERENCES users(id), name TEXT NOT NULL, size BIGINT NOT NULL, expires_at TIMESTAMP, created_at TIMESTAMP);
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

func websiteGroupFlow(t *testing.T, escrow bool) (alice, bob *terminal) {
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
	alice, bob := websiteGroupFlow(t, true)
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
		pool.Exec(`INSERT INTO tracks (title, artist, path, extension, size, uploaded_by) VALUES ($1, 'The Tests', $2, 'mp3', $3, $4)`,
			fmt.Sprintf("Song %d", number), name, len(song), userID)
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
	term.until("the library", func() bool { return strings.Contains(entriesText(term.m.groups[0]), "Song 3") })

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
