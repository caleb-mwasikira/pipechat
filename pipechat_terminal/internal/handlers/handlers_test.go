package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"

	"pipechat/internal/config"
	"pipechat/internal/crypto"
	"pipechat/internal/database"
)

var (
	pubKey = strings.Repeat("0a", 32)
)

// testDatabase makes a fresh schema in the PostgreSQL database named by
// CHAT_TEST_DATABASE_URL and returns a URL whose tables all live in it.
func testDatabase(t *testing.T) string {
	t.Helper()
	base := os.Getenv("CHAT_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("set CHAT_TEST_DATABASE_URL to a PostgreSQL URL to run the server tests")
	}
	admin, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		admin.Close()
	})
	separator := "?"
	if strings.Contains(base, "?") {
		separator = "&"
	}
	return base + separator + "search_path=" + schema
}

// The website's tables, as its Laravel migrations make them (only the columns the server uses).
const websiteSchema = `
CREATE TABLE users (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL DEFAULT '', handle TEXT UNIQUE, bio TEXT, created_at TIMESTAMP DEFAULT now(), updated_at TIMESTAMP, email TEXT NOT NULL UNIQUE, password TEXT NOT NULL);
CREATE TABLE chat_groups (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL, owner_id BIGINT NOT NULL REFERENCES users(id), gid TEXT UNIQUE, escrow BYTEA);
CREATE TABLE chat_group_members (id BIGSERIAL PRIMARY KEY, group_id BIGINT NOT NULL REFERENCES chat_groups(id), user_id BIGINT NOT NULL REFERENCES users(id), delivered BOOLEAN NOT NULL DEFAULT false, UNIQUE (group_id, user_id));
CREATE TABLE chat_group_invites (id BIGSERIAL PRIMARY KEY, group_id BIGINT NOT NULL REFERENCES chat_groups(id), inviter_id BIGINT NOT NULL REFERENCES users(id), invitee_id BIGINT NOT NULL REFERENCES users(id), status TEXT NOT NULL DEFAULT 'pending');
CREATE TABLE tracks (id BIGSERIAL PRIMARY KEY, title TEXT NOT NULL, artist TEXT, path TEXT NOT NULL, extension TEXT NOT NULL, size BIGINT NOT NULL, uploaded_by BIGINT REFERENCES users(id));
CREATE TABLE chat_group_files (id BIGSERIAL PRIMARY KEY, group_id BIGINT NOT NULL REFERENCES chat_groups(id), user_id BIGINT REFERENCES users(id), name TEXT NOT NULL, size BIGINT NOT NULL, expires_at TIMESTAMP, created_at TIMESTAMP);
CREATE TABLE chat_group_grants (group_id BIGINT NOT NULL REFERENCES chat_groups(id), user_id BIGINT NOT NULL REFERENCES users(id), from_user_id BIGINT NOT NULL REFERENCES users(id), sealed BYTEA NOT NULL, created_at TIMESTAMP, PRIMARY KEY (group_id, user_id));`

// useServer starts the routes on a fresh database schema and returns the server's address.
func useServer(t *testing.T) string {
	t.Helper()
	pool, err := database.Connect(context.Background(), testDatabase(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close() })
	if _, err := pool.Exec(websiteSchema); err != nil {
		t.Fatal(err)
	}

	previousPool, previousConfig := Pool, Config
	t.Cleanup(func() { Pool, Config = previousPool, previousConfig })
	Pool = pool
	Config = &config.Config{FilesDir: t.TempDir(), MaxUploadBytes: 1000, FileDays: 7}
	live.sessions = map[string]*session{}
	live.members = map[string]map[*session]bool{}
	live.online = map[string]map[*session]bool{}
	failures.counts = map[string][]time.Time{}

	server := httptest.NewServer(Routes())
	t.Cleanup(server.Close)
	return server.URL
}

func call(t *testing.T, method string, url string, token string, body any, into any) int {
	t.Helper()
	var reader io.Reader
	if raw, ok := body.([]byte); ok {
		reader = bytes.NewReader(raw)
	} else if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	request, _ := http.NewRequest(method, url, reader)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if into != nil {
		json.NewDecoder(response.Body).Decode(into)
	}
	return response.StatusCode
}

const testPassword = "correct horse"

// registerUser makes a website user, logs in as them and gives them keys. It returns the session token.
func registerUser(t *testing.T, base string, handle string) string {
	t.Helper()
	hash, _ := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	// Laravel writes $2y$; make sure that spelling works.
	laravelHash := strings.Replace(string(hash), "$2a$", "$2y$", 1)
	if _, err := Pool.Exec(`INSERT INTO users (handle, email, password) VALUES ($1, $2, $3)`, handle, handle+"@example.com", laravelHash); err != nil {
		t.Fatal(err)
	}
	var response authResponse
	if status := call(t, "POST", base+"/api/auth/login", "", authRequest{Email: handle + "@example.com", Password: testPassword}, &response); status != 200 || response.Token == "" {
		t.Fatalf("login %s: %d", handle, status)
	}
	if status := call(t, "PUT", base+"/api/identity", response.Token, identityRequest{PubKey: pubKey, Vault: []byte("vault-" + handle)}, nil); status != http.StatusNoContent {
		t.Fatalf("identity %s: %d", handle, status)
	}
	return response.Token
}

func loginAs(t *testing.T, base string, handle string) authResponse {
	t.Helper()
	var response authResponse
	call(t, "POST", base+"/api/auth/login", "", authRequest{Email: handle + "@example.com", Password: testPassword}, &response)
	return response
}

// listen opens the WebSocket and returns the events as they arrive.
func listen(t *testing.T, base string, token string) <-chan event {
	t.Helper()
	header := http.Header{"Authorization": {"Bearer " + token}}
	connection, _, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(base, "http")+"/api/ws", &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		t.Fatalf("websocket: %v", err)
	}
	t.Cleanup(func() { connection.CloseNow() })
	events := make(chan event, 100)
	go func() {
		for {
			var message event
			if err := wsjson.Read(context.Background(), connection, &message); err != nil {
				return
			}
			events <- message
		}
	}()
	return events
}

func next(t *testing.T, events <-chan event) event {
	t.Helper()
	select {
	case message := <-events:
		return message
	case <-time.After(2 * time.Second):
		t.Fatal("no event arrived")
		return event{}
	}
}

func TestAccounts(t *testing.T) {
	base := useServer(t)
	token := registerUser(t, base, "erin")

	if status := call(t, "POST", base+"/api/auth/login", "", authRequest{Email: "erin@example.com", Password: "wrong"}, nil); status != http.StatusUnauthorized {
		t.Fatalf("wrong password: %d", status)
	}
	if status := call(t, "POST", base+"/api/auth/login", "", authRequest{Email: "nobody@example.com", Password: testPassword}, nil); status != http.StatusUnauthorized {
		t.Fatalf("unknown email: %d", status)
	}

	if response := loginAs(t, base, "erin"); response.Handle != "erin" || string(response.Vault) != "vault-erin" || response.PubKey != pubKey {
		t.Fatalf("login: %+v", response)
	}
	if status := call(t, "PUT", base+"/api/vault", token, vaultRequest{Vault: []byte("new")}, nil); status != http.StatusNoContent {
		t.Fatalf("save vault: %d", status)
	}
	if response := loginAs(t, base, "erin"); string(response.Vault) != "new" {
		t.Fatalf("vault not saved: %q", response.Vault)
	}
}

func TestFirstLoginHasNoKeysYet(t *testing.T) {
	base := useServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	Pool.Exec(`INSERT INTO users (handle, email, password) VALUES ('newbie', 'newbie@example.com', $1)`, string(hash))
	first := loginAs(t, base, "newbie")
	if first.Handle != "newbie" || len(first.Vault) != 0 || first.PubKey != "" {
		t.Fatalf("first login: %+v", first)
	}
	if status := call(t, "GET", base+"/api/users/newbie", first.Token, nil, nil); status != http.StatusNotFound {
		t.Fatalf("a user with no keys must not be found: %d", status)
	}
}

func TestRoutesNeedALogin(t *testing.T) {
	base := useServer(t)
	for _, route := range [][2]string{{"PUT", "/api/vault"}, {"GET", "/api/users/x"}, {"POST", "/api/groups"}, {"GET", "/api/ws"}, {"POST", "/api/files"}, {"POST", "/api/groups/0011223344556677/messages"}} {
		if status := call(t, route[0], base+route[1], "", nil, nil); status != http.StatusUnauthorized {
			t.Errorf("%s %s without a token: %d", route[0], route[1], status)
		}
		if status := call(t, route[0], base+route[1], "made-up-token", nil, nil); status != http.StatusUnauthorized {
			t.Errorf("%s %s with a bad token: %d", route[0], route[1], status)
		}
	}
}

func TestLoginIsRateLimited(t *testing.T) {
	base := useServer(t)
	registerUser(t, base, "frank")
	for range failuresLimit {
		call(t, "POST", base+"/api/auth/login", "", authRequest{Email: "frank@example.com", Password: "wrong"}, nil)
	}
	if status := call(t, "POST", base+"/api/auth/login", "", authRequest{Email: "frank@example.com", Password: testPassword}, nil); status != http.StatusTooManyRequests {
		t.Fatalf("expected the limit to apply, got %d", status)
	}
}

func TestGroupMessagesAndHistory(t *testing.T) {
	base := useServer(t)
	alice := registerUser(t, base, "alice")
	bob := registerUser(t, base, "bob")
	id := "00112233445566aa"
	token := strings.Repeat("ab", 32)

	if status := call(t, "POST", base+"/api/groups", alice, createGroupRequest{GID: id, Tok: token}, nil); status != http.StatusCreated {
		t.Fatalf("create: %d", status)
	}
	if status := call(t, "POST", base+"/api/groups", bob, createGroupRequest{GID: id, Tok: token}, nil); status != http.StatusConflict {
		t.Fatalf("duplicate group: %d", status)
	}
	aliceEvents := listen(t, base, alice)

	if status := call(t, "POST", base+"/api/groups/"+id+"/join", bob, joinRequest{Tok: strings.Repeat("cd", 32)}, nil); status != http.StatusForbidden {
		t.Fatalf("wrong token: %d", status)
	}
	if status := call(t, "POST", base+"/api/groups/"+id+"/messages", bob, sendRequest{CT: []byte("x")}, nil); status != http.StatusForbidden {
		t.Fatalf("non-member could send: %d", status)
	}

	call(t, "POST", base+"/api/groups/"+id+"/messages", alice, sendRequest{CT: []byte("first")}, nil)
	if message := next(t, aliceEvents); message.Type != "msg" || string(message.CT) != "first" || message.GID != id {
		t.Fatalf("echo to sender: %+v", message)
	}

	var joined joinResponse
	if status := call(t, "POST", base+"/api/groups/"+id+"/join", bob, joinRequest{Tok: token}, &joined); status != 200 || len(joined.Messages) != 1 || string(joined.Messages[0].CT) != "first" {
		t.Fatalf("join: %d %+v", status, joined)
	}
	bobEvents := listen(t, base, bob)

	call(t, "POST", base+"/api/groups/"+id+"/messages", alice, sendRequest{CT: []byte("second")}, nil)
	live := next(t, bobEvents)
	if string(live.CT) != "second" || live.Seq <= joined.Messages[0].Seq {
		t.Fatalf("live message: %+v", live)
	}

	// Joining again only returns what came after "since".
	call(t, "POST", base+"/api/groups/"+id+"/join", bob, joinRequest{Tok: token, Since: joined.Messages[0].Seq}, &joined)
	if len(joined.Messages) != 1 || string(joined.Messages[0].CT) != "second" {
		t.Fatalf("history since: %+v", joined)
	}

	// After leaving, nothing more arrives and sending is refused.
	call(t, "DELETE", base+"/api/groups/"+id+"/membership", bob, nil, nil)
	if status := call(t, "POST", base+"/api/groups/"+id+"/messages", bob, sendRequest{CT: []byte("x")}, nil); status != http.StatusForbidden {
		t.Fatalf("sent after leaving: %d", status)
	}
	call(t, "POST", base+"/api/groups/"+id+"/messages", alice, sendRequest{CT: []byte("third")}, nil)
	next(t, aliceEvents)
	next(t, aliceEvents)
	select {
	case message := <-bobEvents:
		t.Fatalf("event after leaving: %+v", message)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestGroupsAndHistorySurviveANewSession(t *testing.T) {
	base := useServer(t)
	alice := registerUser(t, base, "alice")
	id := "aabbccddeeff0011"
	token := strings.Repeat("12", 32)
	call(t, "POST", base+"/api/groups", alice, createGroupRequest{GID: id, Tok: token}, nil)
	call(t, "POST", base+"/api/groups/"+id+"/messages", alice, sendRequest{CT: []byte("kept")}, nil)

	// A restart forgets sessions but not the database.
	live.sessions = map[string]*session{}
	live.members = map[string]map[*session]bool{}
	live.online = map[string]map[*session]bool{}
	if status := call(t, "POST", base+"/api/groups/"+id+"/join", alice, joinRequest{Tok: token}, nil); status != http.StatusUnauthorized {
		t.Fatalf("old token still works: %d", status)
	}
	var response authResponse
	response = loginAs(t, base, "alice")
	var joined joinResponse
	if status := call(t, "POST", base+"/api/groups/"+id+"/join", response.Token, joinRequest{Tok: token}, &joined); status != 200 || len(joined.Messages) != 1 || string(joined.Messages[0].CT) != "kept" {
		t.Fatalf("join after restart: %d %+v", status, joined)
	}
}

func TestOldMessagesAreTrimmed(t *testing.T) {
	useServer(t)
	ctx := context.Background()
	database.CreateGroup(ctx, Pool, "0000000000000001", []byte("x"))
	for range 12 {
		if _, err := database.AppendMessage(ctx, Pool, "0000000000000001", []byte("m"), 5); err != nil {
			t.Fatal(err)
		}
	}
	kept, _ := database.MessagesSince(ctx, Pool, "0000000000000001", 0, 100)
	if len(kept) != 5 {
		t.Fatalf("kept %d messages, want 5", len(kept))
	}
	for index := 1; index < len(kept); index++ {
		if kept[index].Seq <= kept[index-1].Seq {
			t.Fatal("sequence numbers must increase")
		}
	}
}

func TestDirectChatNotices(t *testing.T) {
	base := useServer(t)
	alice := registerUser(t, base, "alice")
	bob := registerUser(t, base, "bob")
	bobEvents := listen(t, base, bob)

	var user userResponse
	if status := call(t, "GET", base+"/api/users/bob", alice, nil, &user); status != 200 || user.PubKey != pubKey {
		t.Fatalf("lookup: %d %+v", status, user)
	}
	if status := call(t, "GET", base+"/api/users/nobody", alice, nil, nil); status != http.StatusNotFound {
		t.Fatalf("lookup unknown: %d", status)
	}
	if status := call(t, "POST", base+"/api/direct/alice", alice, nil, nil); status != http.StatusNotFound {
		t.Fatalf("notice to self: %d", status)
	}

	call(t, "POST", base+"/api/direct/bob", alice, nil, nil)
	if notice := next(t, bobEvents); notice.Type != "dm" || notice.Handle != "alice" {
		t.Fatalf("live notice: %+v", notice)
	}
	var response authResponse
	response = loginAs(t, base, "bob")
	if len(response.Peers) != 1 || response.Peers[0] != "alice" {
		t.Fatalf("stored notice: %+v", response.Peers)
	}
	call(t, "DELETE", base+"/api/direct/alice", bob, nil, nil)
	response = authResponse{}
	response = loginAs(t, base, "bob")
	if len(response.Peers) != 0 {
		t.Fatalf("notice not cleared: %+v", response.Peers)
	}
}

func TestFiles(t *testing.T) {
	base := useServer(t)
	alice := registerUser(t, base, "alice")

	var stored fileResponse
	if status := call(t, "POST", base+"/api/files", alice, []byte("ciphertext"), &stored); status != http.StatusCreated || stored.Size != 10 {
		t.Fatalf("upload: %d %+v", status, stored)
	}
	response, _ := http.Get(base + "/api/files/" + stored.ID)
	body, _ := io.ReadAll(response.Body)
	if string(body) != "ciphertext" {
		t.Fatalf("download: %q", body)
	}
	if status := call(t, "GET", base+"/api/files/"+strings.Repeat("0", 32), "", nil, nil); status != http.StatusNotFound {
		t.Fatalf("missing file: %d", status)
	}
	if status := call(t, "GET", base+"/api/files/..%2f..%2fetc%2fpasswd", "", nil, nil); status == 200 {
		t.Fatal("path traversal served a file")
	}
	if status := call(t, "POST", base+"/api/files", alice, bytes.Repeat([]byte("x"), 2000), nil); status != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized upload: %d", status)
	}
	if entries, _ := os.ReadDir(Config.FilesDir); len(entries) != 1 {
		t.Fatalf("leftover files: %d", len(entries))
	}

	// Expired files are removed from disk and from the database.
	expired, _ := database.ExpiredFiles(context.Background(), Pool, time.Now().Add(time.Hour))
	if len(expired) != 1 || expired[0] != stored.ID {
		t.Fatalf("expired: %v", expired)
	}
	os.Remove(filepath.Join(Config.FilesDir, stored.ID))
	database.DeleteFile(context.Background(), Pool, stored.ID)
	if expired, _ := database.ExpiredFiles(context.Background(), Pool, time.Now().Add(time.Hour)); len(expired) != 0 {
		t.Fatalf("not deleted: %v", expired)
	}
}

func TestInvitationsByEmail(t *testing.T) {
	base := useServer(t)
	Config.SMTPHost, Config.SMTPPort, Config.SMTPFrom = "mail.test", "25", "chat@example.com"

	var sent []string
	previous := sendEmail
	t.Cleanup(func() { sendEmail = previous })
	sendEmail = func(address string, _ smtp.Auth, from string, to []string, message []byte) error {
		sent = append(sent, address+"|"+from+"|"+strings.Join(to, ",")+"|"+string(message))
		return nil
	}

	alice := registerUser(t, base, "alice")
	bob := registerUser(t, base, "bob")
	secret, _ := crypto.NewSecret("team")
	call(t, "POST", base+"/api/groups", alice, createGroupRequest{GID: secret.GIDHex(), Tok: secret.JoinToken()}, nil)
	invite := func(token string, email string, code string) int {
		return call(t, "POST", base+"/api/invitations", token, inviteRequest{Email: email, Code: code}, nil)
	}

	if status := call(t, "POST", base+"/api/invitations", "", inviteRequest{}, nil); status != http.StatusUnauthorized {
		t.Fatalf("not logged in: %d", status)
	}
	if status := invite(alice, "not an email", secret.Code()); status != http.StatusBadRequest {
		t.Fatalf("bad address: %d", status)
	}
	if status := invite(alice, "friend@example.com", "garbage"); status != http.StatusBadRequest {
		t.Fatalf("bad code: %d", status)
	}
	other, _ := crypto.NewSecret("not-a-group")
	if status := invite(alice, "friend@example.com", other.Code()); status != http.StatusForbidden {
		t.Fatalf("code of a group that does not exist: %d", status)
	}
	if status := invite(bob, "friend@example.com", secret.Code()); status != http.StatusForbidden {
		t.Fatalf("non-member could invite: %d", status)
	}
	if len(sent) != 0 {
		t.Fatal("email sent for a refused request")
	}

	if status := invite(alice, "Friend <friend@example.com>", secret.Code()); status != http.StatusNoContent {
		t.Fatalf("invite: %d", status)
	}
	if len(sent) != 1 || !strings.Contains(sent[0], "mail.test:25|chat@example.com|friend@example.com|") ||
		!strings.Contains(sent[0], secret.Code()) || !strings.Contains(sent[0], "@alice invited you") {
		t.Fatalf("email: %q", sent)
	}

	for range invitesPerHour {
		invite(alice, "friend@example.com", secret.Code())
	}
	if status := invite(alice, "friend@example.com", secret.Code()); status != http.StatusTooManyRequests {
		t.Fatalf("no rate limit: %d", status)
	}

	Config.SMTPHost = ""
	if status := invite(alice, "friend@example.com", secret.Code()); status != http.StatusServiceUnavailable {
		t.Fatalf("without SMTP: %d", status)
	}
}

// A group made on the website: the owner's terminal creates it, a member's terminal
// seals its key for the invitee, and the invitee's terminal receives it.
func TestWebsiteGroups(t *testing.T) {
	base := useServer(t)
	alice := registerUser(t, base, "alice")
	bob := registerUser(t, base, "bob")
	var aliceID, bobID, group int64
	Pool.QueryRow(`SELECT id FROM users WHERE handle = 'alice'`).Scan(&aliceID)
	Pool.QueryRow(`SELECT id FROM users WHERE handle = 'bob'`).Scan(&bobID)
	Pool.QueryRow(`INSERT INTO chat_groups (name, owner_id) VALUES ('crew', $1) RETURNING id`, aliceID).Scan(&group)
	Pool.Exec(`INSERT INTO chat_group_members (group_id, user_id) VALUES ($1, $2)`, group, aliceID)

	sync := func(token string) database.Sync {
		var result database.Sync
		call(t, "GET", base+"/api/sync", token, nil, &result)
		return result
	}
	if got := sync(alice); len(got.Create) != 1 || got.Create[0].Name != "crew" {
		t.Fatalf("owner should be told to create the group: %+v", got)
	}
	if got := sync(bob); len(got.Create) != 0 {
		t.Fatalf("only the owner creates: %+v", got)
	}

	const gid = "00000000000000aa"
	if status := call(t, "POST", base+"/api/sync/groups", alice, gidRequest{Group: group, GID: gid}, nil); status != http.StatusNotFound {
		t.Fatalf("group not created on the chat server yet: %d", status)
	}
	call(t, "POST", base+"/api/groups", alice, createGroupRequest{GID: gid, Tok: strings.Repeat("ab", 32)}, nil)
	if status := call(t, "POST", base+"/api/sync/groups", bob, gidRequest{Group: group, GID: gid}, nil); status != http.StatusForbidden {
		t.Fatalf("a non-owner must not set the id: %d", status)
	}
	if status := call(t, "POST", base+"/api/sync/groups", alice, gidRequest{Group: group, GID: gid}, nil); status != http.StatusNoContent {
		t.Fatalf("set id: %d", status)
	}

	// Bob accepts an invitation on the website.
	Pool.Exec(`INSERT INTO chat_group_members (group_id, user_id) VALUES ($1, $2)`, group, bobID)
	waiting := sync(alice).Grant
	if len(waiting) != 1 || waiting[0].Handle != "bob" || waiting[0].GID != gid {
		t.Fatalf("alice should be told bob waits: %+v", waiting)
	}
	if status := call(t, "POST", base+"/api/sync/grants", bob, grantRequest{Group: group, Handle: "bob", Sealed: []byte("x")}, nil); status != http.StatusConflict {
		t.Fatalf("someone without the key cannot grant: %d", status)
	}
	if status := call(t, "POST", base+"/api/sync/grants", alice, grantRequest{Group: group, Handle: "bob", Sealed: []byte("sealed")}, nil); status != http.StatusNoContent {
		t.Fatalf("grant: %d", status)
	}
	if got := sync(alice).Grant; len(got) != 0 {
		t.Fatalf("grant already sent: %+v", got)
	}
	incoming := sync(bob).Incoming
	if len(incoming) != 1 || incoming[0].From != "alice" || string(incoming[0].Sealed) != "sealed" || incoming[0].Name != "crew" {
		t.Fatalf("bob should receive the key: %+v", incoming)
	}
	call(t, "DELETE", base+"/api/sync/grants/"+strconv.FormatInt(group, 10), bob, nil, nil)
	if got := sync(bob); len(got.Incoming) != 0 || len(sync(alice).Grant) != 0 {
		t.Fatalf("key should be delivered: %+v", got)
	}

	// New keys (a password reset) mean the group must be sent again.
	call(t, "PUT", base+"/api/identity", bob, identityRequest{PubKey: strings.Repeat("0b", 32), Vault: []byte("fresh")}, nil)
	if got := sync(alice).Grant; len(got) != 1 {
		t.Fatalf("after new keys alice should share again: %+v", got)
	}
}

func TestEscrowHandsKeysOverWithoutAMemberOnline(t *testing.T) {
	base := useServer(t)
	Config.EscrowKey = bytes.Repeat([]byte{9}, 32)
	alice := registerUser(t, base, "alice")
	bob := registerUser(t, base, "bob")
	var aliceID, bobID, group int64
	Pool.QueryRow(`SELECT id FROM users WHERE handle = 'alice'`).Scan(&aliceID)
	Pool.QueryRow(`SELECT id FROM users WHERE handle = 'bob'`).Scan(&bobID)

	secret, _ := crypto.NewSecret("crew")
	Pool.QueryRow(`INSERT INTO chat_groups (name, owner_id, gid) VALUES ('crew', $1, $2) RETURNING id`, aliceID, secret.GIDHex()).Scan(&group)
	Pool.Exec(`INSERT INTO chat_group_members (group_id, user_id, delivered) VALUES ($1, $2, true)`, group, aliceID)
	call(t, "POST", base+"/api/groups", alice, createGroupRequest{GID: secret.GIDHex(), Tok: secret.JoinToken()}, nil)

	var sync database.Sync
	call(t, "GET", base+"/api/sync", alice, nil, &sync)
	serverKey, err := crypto.ParsePublicKey(sync.ServerKey)
	if err != nil || len(sync.NeedEscrow) != 1 {
		t.Fatalf("alice should be asked for the key: %+v", sync)
	}

	// A key for another group, or from a non-member, is refused.
	other, _ := crypto.NewSecret("other")
	wrong, _ := crypto.SealAnon(serverKey, other)
	if status := call(t, "POST", base+"/api/sync/escrow", alice, escrowRequest{Group: group, Sealed: wrong}, nil); status != http.StatusBadRequest {
		t.Fatalf("wrong group key: %d", status)
	}
	sealed, _ := crypto.SealAnon(serverKey, secret)
	if status := call(t, "POST", base+"/api/sync/escrow", bob, escrowRequest{Group: group, Sealed: sealed}, nil); status != http.StatusConflict {
		t.Fatalf("non-member: %d", status)
	}
	if status := call(t, "POST", base+"/api/sync/escrow", alice, escrowRequest{Group: group, Sealed: sealed}, nil); status != http.StatusNoContent {
		t.Fatalf("escrow: %d", status)
	}
	sync = database.Sync{}
	call(t, "GET", base+"/api/sync", alice, nil, &sync)
	if len(sync.NeedEscrow) != 0 {
		t.Fatalf("already held: %+v", sync.NeedEscrow)
	}

	// Bob joins with no other member online; his own sync carries the key, sealed to him.
	Pool.Exec(`INSERT INTO chat_group_members (group_id, user_id) VALUES ($1, $2)`, group, bobID)
	sync = database.Sync{}
	call(t, "GET", base+"/api/sync", bob, nil, &sync)
	if len(sync.Incoming) != 1 || sync.Incoming[0].FromKey != sync.ServerKey {
		t.Fatalf("bob should get the key from the server: %+v", sync.Incoming)
	}
}

func TestCreatorCanRemoveAMember(t *testing.T) {
	base := useServer(t)
	alice := registerUser(t, base, "alice")
	bob := registerUser(t, base, "bob")
	carol := registerUser(t, base, "carol")
	var ids = map[string]int64{}
	for _, handle := range []string{"alice", "bob", "carol"} {
		var id int64
		Pool.QueryRow(`SELECT id FROM users WHERE handle = $1`, handle).Scan(&id)
		ids[handle] = id
	}
	const gid = "00000000000000bb"
	token := strings.Repeat("cd", 32)
	var group int64
	Pool.QueryRow(`INSERT INTO chat_groups (name, owner_id, gid) VALUES ('crew', $1, $2) RETURNING id`, ids["alice"], gid).Scan(&group)
	for _, handle := range []string{"alice", "bob"} {
		Pool.Exec(`INSERT INTO chat_group_members (group_id, user_id, delivered) VALUES ($1, $2, true)`, group, ids[handle])
	}
	call(t, "POST", base+"/api/groups", alice, createGroupRequest{GID: gid, Tok: token}, nil)
	for _, member := range []string{alice, bob} {
		if status := call(t, "POST", base+"/api/groups/"+gid+"/join", member, joinRequest{Tok: token}, nil); status != 200 {
			t.Fatalf("member join: %d", status)
		}
	}
	bobEvents := listen(t, base, bob)

	kick := func(token string, handle string) int {
		return call(t, "POST", base+"/api/sync/kick", token, kickRequest{GID: gid, Handle: handle}, nil)
	}
	if status := kick(bob, "alice"); status != http.StatusForbidden {
		t.Fatalf("a member removing the creator: %d", status)
	}
	if status := kick(carol, "bob"); status != http.StatusForbidden {
		t.Fatalf("an outsider removing a member: %d", status)
	}
	if status := kick(alice, "alice"); status != http.StatusConflict {
		t.Fatalf("removing the creator: %d", status)
	}
	if status := kick(alice, "carol"); status != http.StatusConflict {
		t.Fatalf("removing a non-member: %d", status)
	}
	if status := kick(alice, "bob"); status != http.StatusNoContent {
		t.Fatalf("remove: %d", status)
	}

	if message := next(t, bobEvents); message.Type != "kick" || message.GID != gid {
		t.Fatalf("bob should be told: %+v", message)
	}
	var count int
	Pool.QueryRow(`SELECT COUNT(*) FROM chat_group_members WHERE group_id = $1`, group).Scan(&count)
	if count != 1 {
		t.Fatalf("members left: %d", count)
	}
	// Having the key is no longer enough.
	if status := call(t, "POST", base+"/api/groups/"+gid+"/join", bob, joinRequest{Tok: token}, nil); status != http.StatusForbidden {
		t.Fatalf("join after removal: %d", status)
	}
	if status := call(t, "POST", base+"/api/groups/"+gid+"/messages", bob, sendRequest{CT: []byte("hi")}, nil); status != http.StatusForbidden {
		t.Fatalf("send after removal: %d", status)
	}
	if status := call(t, "POST", base+"/api/groups/"+gid+"/messages", alice, sendRequest{CT: []byte("hi")}, nil); status != http.StatusNoContent {
		t.Fatalf("the creator can still send: %d", status)
	}
}

func TestMusicLibrary(t *testing.T) {
	base := useServer(t)
	Config.StorageDir = t.TempDir()
	token := registerUser(t, base, "djay")
	os.MkdirAll(filepath.Join(Config.StorageDir, "music"), 0o755)
	os.WriteFile(filepath.Join(Config.StorageDir, "music", "a.mp3"), []byte("0123456789"), 0o644)
	os.WriteFile(filepath.Join(Config.StorageDir, "secret.txt"), []byte("do not serve"), 0o644)
	var userID int64
	Pool.QueryRow(`SELECT id FROM users WHERE handle = 'djay'`).Scan(&userID)
	Pool.Exec(`INSERT INTO tracks (title, artist, path, extension, size, uploaded_by) VALUES ('One', NULL, 'music/a.mp3', 'mp3', 10, $1)`, userID)
	Pool.Exec(`INSERT INTO tracks (title, path, extension, size) VALUES ('Sneaky', '../../etc/passwd', 'mp3', 1)`)
	Pool.Exec(`INSERT INTO tracks (title, path, extension, size) VALUES ('Gone', 'music/missing.mp3', 'mp3', 1)`)

	if status := call(t, "GET", base+"/api/music", "", nil, nil); status != http.StatusUnauthorized {
		t.Fatalf("list without a login: %d", status)
	}
	if status := call(t, "GET", base+"/api/music/1/file", "", nil, nil); status != http.StatusUnauthorized {
		t.Fatalf("file without a login: %d", status)
	}
	var tracks []database.Track
	if status := call(t, "GET", base+"/api/music", token, nil, &tracks); status != 200 || len(tracks) != 3 || tracks[2].Title != "One" || tracks[2].Uploader != "djay" {
		t.Fatalf("list: %d %+v", status, tracks)
	}

	get := func(path string, rangeHeader string) (int, string, string) {
		request, _ := http.NewRequest("GET", base+path, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		if rangeHeader != "" {
			request.Header.Set("Range", rangeHeader)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, _ := io.ReadAll(response.Body)
		return response.StatusCode, string(body), response.Header.Get("Content-Type")
	}
	if status, body, kind := get("/api/music/1/file", ""); status != 200 || body != "0123456789" || kind != "audio/mpeg" {
		t.Fatalf("file: %d %q %q", status, body, kind)
	}
	if status, body, _ := get("/api/music/1/file", "bytes=2-4"); status != http.StatusPartialContent || body != "234" {
		t.Fatalf("range: %d %q", status, body)
	}
	if status, _, _ := get("/api/music/2/file", ""); status != http.StatusNotFound {
		t.Fatalf("a path outside the music folder: %d", status)
	}
	if status, _, _ := get("/api/music/3/file", ""); status != http.StatusNotFound {
		t.Fatalf("a missing file: %d", status)
	}
	if status, _, _ := get("/api/music/99/file", ""); status != http.StatusNotFound {
		t.Fatalf("no such track: %d", status)
	}
	if status, _, _ := get("/api/music/abc/file", ""); status != http.StatusBadRequest {
		t.Fatalf("bad id: %d", status)
	}
}

func TestBioShowsInWhois(t *testing.T) {
	base := useServer(t)
	alice := registerUser(t, base, "alice")
	bob := registerUser(t, base, "bob")

	var profile userResponse
	if status := call(t, "GET", base+"/api/users/alice", bob, nil, &profile); status != 200 || profile.Bio != "" || profile.PubKey == "" || profile.Joined == 0 {
		t.Fatalf("profile: %d %+v", status, profile)
	}
	if profile.Online {
		t.Fatal("alice has no connection open, so she is not online")
	}

	if status := call(t, "PUT", base+"/api/profile", alice, bioRequest{Bio: "  Hello\n\tthere\x07 friend  "}, nil); status != http.StatusNoContent {
		t.Fatalf("save bio: %d", status)
	}
	listen(t, base, alice)
	for range 50 { // the server notes the open socket a moment after the client sees it connect
		call(t, "GET", base+"/api/users/alice", bob, nil, &profile)
		if profile.Online {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if profile.Bio != "Hello there friend" || !profile.Online {
		t.Fatalf("bio %q, online %v", profile.Bio, profile.Online)
	}

	if status := call(t, "PUT", base+"/api/profile", alice, bioRequest{Bio: strings.Repeat("x", 281)}, nil); status != http.StatusBadRequest {
		t.Fatalf("too long: %d", status)
	}
	if status := call(t, "PUT", base+"/api/profile", alice, bioRequest{Bio: ""}, nil); status != http.StatusNoContent {
		t.Fatalf("clear bio: %d", status)
	}
	profile = userResponse{} // a field the server leaves out would keep its old value
	call(t, "GET", base+"/api/users/alice", bob, nil, &profile)
	if profile.Bio != "" {
		t.Fatalf("bio should be cleared: %q", profile.Bio)
	}
	if status := call(t, "PUT", base+"/api/profile", "", bioRequest{Bio: "x"}, nil); status != http.StatusUnauthorized {
		t.Fatalf("without a login: %d", status)
	}
}

func TestWhoIsOnlineInAChat(t *testing.T) {
	base := useServer(t)
	alice := registerUser(t, base, "alice")
	bob := registerUser(t, base, "bob")
	carol := registerUser(t, base, "carol")
	const gid = "00000000000000cc"
	token := strings.Repeat("ab", 32)
	call(t, "POST", base+"/api/groups", alice, createGroupRequest{GID: gid, Tok: token}, nil)
	for _, member := range []string{alice, bob, carol} {
		call(t, "POST", base+"/api/groups/"+gid+"/join", member, joinRequest{Tok: token}, nil)
	}

	online := func(asking string) (int, []string) {
		var handles []string
		status := call(t, "GET", base+"/api/groups/"+gid+"/online", asking, nil, &handles)
		return status, handles
	}
	if status, handles := online(alice); status != 200 || len(handles) != 0 {
		t.Fatalf("nobody has a socket open yet: %d %v", status, handles)
	}

	listen(t, base, alice)
	listen(t, base, bob) // carol joined but has no socket: her terminal is closed
	var handles []string
	for range 50 {
		_, handles = online(alice)
		if len(handles) == 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(handles) != 2 || handles[0] != "alice" || handles[1] != "bob" {
		t.Fatalf("online: %v", handles)
	}

	outsider := registerUser(t, base, "dave")
	if status, _ := online(outsider); status != http.StatusForbidden {
		t.Fatalf("someone not in the chat must not see who is: %d", status)
	}
}

func TestSharesAndPresenceAreRecordedForTheWebsite(t *testing.T) {
	base := useServer(t)
	Config.FileDays = 7
	alice := registerUser(t, base, "alice")
	bob := registerUser(t, base, "bob")
	var aliceID, group int64
	Pool.QueryRow(`SELECT id FROM users WHERE handle = 'alice'`).Scan(&aliceID)
	const gid = "00000000000000dd"
	Pool.QueryRow(`INSERT INTO chat_groups (name, owner_id, gid) VALUES ('crew', $1, $2) RETURNING id`, aliceID, gid).Scan(&group)
	Pool.Exec(`INSERT INTO chat_group_members (group_id, user_id, delivered) VALUES ($1, $2, true)`, group, aliceID)

	share := func(token string, id string) int {
		return call(t, "POST", base+"/api/sync/shares", token, shareRequest{GID: id, Name: "holiday.zip", Size: 1234}, nil)
	}
	if status := share(alice, gid); status != http.StatusNoContent {
		t.Fatalf("share: %d", status)
	}
	if status := share(bob, gid); status != http.StatusNotFound {
		t.Fatalf("a non-member's share must not be listed: %d", status)
	}
	if status := share(alice, "00000000000000ee"); status != http.StatusNotFound {
		t.Fatalf("not a website group: %d", status)
	}
	var count int
	var days float64
	Pool.QueryRow(`SELECT COUNT(*), COALESCE(MAX(EXTRACT(EPOCH FROM (expires_at - created_at)) / 86400), 0) FROM chat_group_files WHERE group_id = $1 AND name = 'holiday.zip' AND size = 1234`, group).Scan(&count, &days)
	if count != 1 || days < 6.9 || days > 7.1 {
		t.Fatalf("recorded %d, expires in %.1f days", count, days)
	}

	// Presence follows the open socket, for the website to read.
	present := func() (bool, int64) {
		var online bool
		var seen int64
		Pool.QueryRow(`SELECT online, last_seen FROM presence WHERE handle = 'alice'`).Scan(&online, &seen)
		return online, seen
	}
	if online, _ := present(); online {
		t.Fatal("not online before any socket")
	}
	connection := listenAndKeep(t, base, alice)
	for range 50 {
		if online, _ := present(); online {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if online, seen := present(); !online || time.Now().Unix()-seen > 5 {
		t.Fatalf("online %v, last seen %d", online, seen)
	}
	connection.Close(websocket.StatusNormalClosure, "bye")
	for range 100 {
		if online, _ := present(); !online {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("closing the socket should mark the person offline")
}

// listenAndKeep opens the WebSocket and hands back the connection so a test can close it.
func listenAndKeep(t *testing.T, base string, token string) *websocket.Conn {
	t.Helper()
	header := http.Header{"Authorization": {"Bearer " + token}}
	connection, _, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(base, "http")+"/api/ws", &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		t.Fatalf("websocket: %v", err)
	}
	t.Cleanup(func() { connection.CloseNow() })
	return connection
}
