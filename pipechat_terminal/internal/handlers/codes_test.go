package handlers

import (
	"net/http"
	"testing"
)

func TestJoinWithCode(t *testing.T) {
	base := useServer(t)
	alice := registerUser(t, base, "alice")
	bob := registerUser(t, base, "bob")
	var aliceID, group int64
	Pool.QueryRow(`SELECT id FROM users WHERE handle = 'alice'`).Scan(&aliceID)
	Pool.QueryRow(`INSERT INTO chat_groups (name, owner_id) VALUES ('crew', $1) RETURNING id`, aliceID).Scan(&group)
	Pool.Exec(`INSERT INTO chat_group_members (group_id, user_id) VALUES ($1, $2)`, group, aliceID)
	Pool.Exec(`INSERT INTO chat_group_invite_codes (group_id, code, max_uses, expires_at) VALUES ($1, 'ABCD2345', 1, now() + interval '1 day')`, group)
	Pool.Exec(`INSERT INTO chat_group_invite_codes (group_id, code, expires_at) VALUES ($1, 'OLDOLD22', now() - interval '1 day')`, group)

	var result struct {
		Name  string `json:"name"`
		Added bool   `json:"added"`
	}
	if status := call(t, "POST", base+"/api/join/code", bob, map[string]string{"code": "oldold22"}, nil); status != http.StatusNotFound {
		t.Fatalf("expired code: %d", status)
	}
	if status := call(t, "POST", base+"/api/join/code", bob, map[string]string{"code": "abcd-2345"}, &result); status != http.StatusOK || result.Name != "crew" || !result.Added {
		t.Fatalf("good code: %d %+v", status, result)
	}
	var members int
	Pool.QueryRow(`SELECT count(*) FROM chat_group_members WHERE group_id = $1`, group).Scan(&members)
	if members != 2 {
		t.Fatalf("bob should be a member: %d", members)
	}
	// Used up for newcomers, but the member himself still gets a friendly answer: he is in, so it was not spent twice.
	if status := call(t, "POST", base+"/api/join/code", alice, map[string]string{"code": "ABCD2345"}, nil); status != http.StatusNotFound {
		t.Fatalf("used up code: %d", status)
	}
}
