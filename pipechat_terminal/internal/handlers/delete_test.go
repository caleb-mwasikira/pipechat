package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestOnlyTheUploaderOrTheGroupOwnerDeletesAFile(t *testing.T) {
	base := useServer(t)
	alice := registerUser(t, base, "alice") // owns the group
	bob := registerUser(t, base, "bob")     // uploads
	carol := registerUser(t, base, "carol") // a member, nothing more
	var aliceID, bobID, group int64
	Pool.QueryRow(`SELECT id FROM users WHERE handle = 'alice'`).Scan(&aliceID)
	Pool.QueryRow(`SELECT id FROM users WHERE handle = 'bob'`).Scan(&bobID)
	Pool.QueryRow(`INSERT INTO chat_groups (name, owner_id, gid) VALUES ('crew', $1, '00000000000000aa') RETURNING id`, aliceID).Scan(&group)

	stored := func(id string, uploader string) {
		os.WriteFile(filepath.Join(Config.FilesDir, id), []byte("ciphertext"), 0o644)
		Pool.Exec(`INSERT INTO files (id, handle, size, created_at) VALUES ($1, $2, 10, 1)`, id, uploader)
		Pool.Exec(`INSERT INTO chat_group_files (group_id, user_id, name, size, file_id) VALUES ($1, $2, 'x', 10, $3)`, group, bobID, id)
	}
	exists := func(id string) bool { _, err := os.Stat(filepath.Join(Config.FilesDir, id)); return err == nil }

	mine, theirs := strings.Repeat("a", 32), strings.Repeat("b", 32)
	stored(mine, "bob")
	stored(theirs, "bob")

	if status := call(t, "DELETE", base+"/api/files/"+mine, carol, nil, nil); status != http.StatusForbidden || !exists(mine) {
		t.Fatalf("a stranger must not delete: %d", status)
	}
	if status := call(t, "DELETE", base+"/api/files/"+mine, bob, nil, nil); status != http.StatusNoContent || exists(mine) {
		t.Fatalf("the uploader deletes: %d", status)
	}
	var left int
	Pool.QueryRow(`SELECT COUNT(*) FROM chat_group_files WHERE file_id = $1`, mine).Scan(&left)
	if left != 0 {
		t.Fatal("the website listing goes with the file")
	}
	if status := call(t, "DELETE", base+"/api/files/"+theirs, alice, nil, nil); status != http.StatusNoContent || exists(theirs) {
		t.Fatalf("the group's owner deletes: %d", status)
	}
	if status := call(t, "DELETE", base+"/api/files/"+mine, bob, nil, nil); status != http.StatusNotFound {
		t.Fatalf("already gone: %d", status)
	}
}

func TestOnlyTheUploaderOrAnAdminDeletesASong(t *testing.T) {
	base := useServer(t)
	Config.StorageDir = t.TempDir()
	os.MkdirAll(filepath.Join(Config.StorageDir, "music"), 0o755)
	alice := registerUser(t, base, "alice")
	bob := registerUser(t, base, "bob")
	boss := registerUser(t, base, "boss")
	var aliceID int64
	Pool.QueryRow(`SELECT id FROM users WHERE handle = 'alice'`).Scan(&aliceID)
	Pool.Exec(`UPDATE users SET is_admin = true WHERE handle = 'boss'`)

	add := func(name string) int64 {
		os.WriteFile(filepath.Join(Config.StorageDir, "music", name), []byte("la"), 0o644)
		var id int64
		Pool.QueryRow(`INSERT INTO tracks (title, path, extension, size, uploaded_by) VALUES ($1, $2, 'mp3', 2, $3) RETURNING id`, name, "music/"+name, aliceID).Scan(&id)
		return id
	}
	present := func(name string) bool {
		_, err := os.Stat(filepath.Join(Config.StorageDir, "music", name))
		return err == nil
	}
	url := func(id int64) string { return base + "/api/music/" + strconv.FormatInt(id, 10) }

	first, second := add("one.mp3"), add("two.mp3")
	if status := call(t, "DELETE", url(first), bob, nil, nil); status != http.StatusForbidden || !present("one.mp3") {
		t.Fatalf("someone else must not delete: %d", status)
	}
	if status := call(t, "DELETE", url(first), alice, nil, nil); status != http.StatusNoContent || present("one.mp3") {
		t.Fatalf("the uploader deletes: %d", status)
	}
	if status := call(t, "DELETE", url(second), boss, nil, nil); status != http.StatusNoContent || present("two.mp3") {
		t.Fatalf("an administrator deletes: %d", status)
	}
	if status := call(t, "DELETE", url(second), boss, nil, nil); status != http.StatusNotFound {
		t.Fatalf("already gone: %d", status)
	}
}
