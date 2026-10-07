package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrNotWebGroup = errors.New("that group was not made on the website")
	ErrNotOwner    = errors.New("only the group's creator can remove members")
	ErrNotMember   = errors.New("that user is not in the group")
	ErrKickOwner   = errors.New("the creator cannot be removed")
)

// The website owns these tables: users, chat_groups, chat_group_members and
// chat_group_grants (see database/migrations in the Laravel app). The chat server
// reads them to log people in and to hand group keys from member to member.

// WebUser is a website account.
type WebUser struct {
	ID           int64
	Handle       string
	PasswordHash string
}

// GetWebUser returns nil when no website user has that email.
func GetWebUser(ctx context.Context, pool *sql.DB, email string) (*WebUser, error) {
	user := &WebUser{}
	var handle sql.NullString
	err := pool.QueryRowContext(ctx, `SELECT id, handle, password FROM users WHERE lower(email) = lower($1)`, email).
		Scan(&user.ID, &handle, &user.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	user.Handle = handle.String
	return user, err
}

// AccountForUser returns the chat account linked to a website user, or nil.
func AccountForUser(ctx context.Context, pool *sql.DB, user *WebUser) (*Account, error) {
	account := &Account{}
	err := pool.QueryRowContext(ctx, `SELECT handle, pubkey, vault FROM accounts WHERE user_id = $1`, user.ID).
		Scan(&account.Handle, &account.PubKey, &account.Vault)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return account, err
}

// LinkAccount creates the chat account of a website user on their first terminal login.
// It has no keys yet; the terminal makes them and sends them with SetIdentity.
func LinkAccount(ctx context.Context, pool *sql.DB, user *WebUser) (bool, error) {
	result, err := pool.ExecContext(ctx,
		`INSERT INTO accounts (handle, auth_hash, pubkey, vault, created_at, user_id)
		 VALUES ($1, '', '', ''::bytea, extract(epoch FROM now())::bigint, $2) ON CONFLICT DO NOTHING`, user.Handle, user.ID)
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows == 1, nil
}

// SetIdentity stores a new public key and vault. Everyone's copy of the group keys
// was sealed to the old key, so this user's groups are marked as not delivered:
// another member's terminal will send them again.
func SetIdentity(ctx context.Context, pool *sql.DB, handle string, publicKey string, vault []byte) error {
	transaction, err := pool.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	var userID int64
	var oldKey string
	if err := transaction.QueryRowContext(ctx, `SELECT user_id, pubkey FROM accounts WHERE handle = $1 FOR UPDATE`, handle).Scan(&userID, &oldKey); err != nil {
		return err
	}
	if _, err := transaction.ExecContext(ctx, `UPDATE accounts SET pubkey = $1, vault = $2 WHERE handle = $3`, publicKey, vault, handle); err != nil {
		return err
	}
	if oldKey != publicKey {
		if _, err := transaction.ExecContext(ctx, `UPDATE chat_group_members SET delivered = false WHERE user_id = $1`, userID); err != nil {
			return err
		}
		if _, err := transaction.ExecContext(ctx, `DELETE FROM chat_group_grants WHERE user_id = $1`, userID); err != nil {
			return err
		}
	}
	return transaction.Commit()
}

// ---- syncing website groups to a terminal ----

type NewGroup struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// ToGrant is a member waiting for a group's key.
type ToGrant struct {
	Group  int64  `json:"group"`
	GID    string `json:"gid"`
	Handle string `json:"handle"`
	PubKey string `json:"pubkey"`
}

// Incoming is a group key sealed to this user.
type Incoming struct {
	Group   int64  `json:"group"`
	Name    string `json:"name"`
	From    string `json:"from"`
	FromKey string `json:"from_key"`
	Sealed  []byte `json:"sealed"`
}

// EscrowedGroup is a group this user waits for, whose key the server holds.
type EscrowedGroup struct {
	Group  int64
	Name   string
	Escrow []byte
}

// MissingEscrow is a group this user has the key to, which the server does not yet hold.
type MissingEscrow struct {
	Group int64  `json:"group"`
	GID   string `json:"gid"`
}

// GroupName is what the website calls one of this user's groups now. The creator can rename a group there,
// and terminals use this to show the new name.
type GroupName struct {
	GID  string `json:"gid"`
	Name string `json:"name"`
}

type Sync struct {
	Names      []GroupName     `json:"names"`
	ServerKey  string          `json:"server_key,omitempty"` // the server's public key, when it holds group keys
	NeedEscrow []MissingEscrow `json:"need_escrow"`          // groups the terminal should upload the key of
	Escrowed   []EscrowedGroup `json:"-"`
	Create     []NewGroup      `json:"create"`   // groups this user owns that still need creating
	Grant      []ToGrant       `json:"grant"`    // members of this user's groups waiting for the key
	Incoming   []Incoming      `json:"incoming"` // keys sent to this user
}

func rowsOf[T any](rows *sql.Rows, err error, scan func(*sql.Rows) (T, error)) ([]T, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// GetSync collects everything the website wants this user's terminal to do.
func GetSync(ctx context.Context, pool *sql.DB, userID int64) (*Sync, error) {
	var out Sync
	var err error
	out.Create, err = func() ([]NewGroup, error) {
		rows, err := pool.QueryContext(ctx, `SELECT id, name FROM chat_groups WHERE owner_id = $1 AND gid IS NULL`, userID)
		return rowsOf(rows, err, func(r *sql.Rows) (g NewGroup, err error) { err = r.Scan(&g.ID, &g.Name); return })
	}()
	if err != nil {
		return nil, err
	}
	out.Grant, err = func() ([]ToGrant, error) {
		rows, err := pool.QueryContext(ctx, `
			SELECT g.id, g.gid, a.handle, a.pubkey
			FROM chat_groups g
			JOIN chat_group_members me ON me.group_id = g.id AND me.user_id = $1 AND me.delivered
			JOIN chat_group_members m ON m.group_id = g.id AND m.user_id <> $1 AND NOT m.delivered
			JOIN accounts a ON a.user_id = m.user_id AND a.pubkey <> ''
			WHERE g.gid IS NOT NULL
			  AND NOT EXISTS (SELECT 1 FROM chat_group_grants x WHERE x.group_id = g.id AND x.user_id = m.user_id)`, userID)
		return rowsOf(rows, err, func(r *sql.Rows) (g ToGrant, err error) { err = r.Scan(&g.Group, &g.GID, &g.Handle, &g.PubKey); return })
	}()
	if err != nil {
		return nil, err
	}
	out.Incoming, err = func() ([]Incoming, error) {
		rows, err := pool.QueryContext(ctx, `
			SELECT g.id, g.name, a.handle, a.pubkey, x.sealed
			FROM chat_group_grants x
			JOIN chat_groups g ON g.id = x.group_id
			JOIN accounts a ON a.user_id = x.from_user_id
			WHERE x.user_id = $1`, userID)
		return rowsOf(rows, err, func(r *sql.Rows) (g Incoming, err error) {
			err = r.Scan(&g.Group, &g.Name, &g.From, &g.FromKey, &g.Sealed)
			return
		})
	}()
	if err != nil {
		return nil, err
	}
	out.Names, err = func() ([]GroupName, error) {
		rows, err := pool.QueryContext(ctx, `
			SELECT g.gid, g.name FROM chat_groups g
			JOIN chat_group_members me ON me.group_id = g.id AND me.user_id = $1
			WHERE g.gid IS NOT NULL`, userID)
		return rowsOf(rows, err, func(r *sql.Rows) (n GroupName, err error) { err = r.Scan(&n.GID, &n.Name); return })
	}()
	if err != nil {
		return nil, err
	}
	out.NeedEscrow, err = func() ([]MissingEscrow, error) {
		rows, err := pool.QueryContext(ctx, `
			SELECT g.id, g.gid FROM chat_groups g
			JOIN chat_group_members me ON me.group_id = g.id AND me.user_id = $1 AND me.delivered
			WHERE g.gid IS NOT NULL AND g.escrow IS NULL`, userID)
		return rowsOf(rows, err, func(r *sql.Rows) (g MissingEscrow, err error) { err = r.Scan(&g.Group, &g.GID); return })
	}()
	if err != nil {
		return nil, err
	}
	out.Escrowed, err = func() ([]EscrowedGroup, error) {
		rows, err := pool.QueryContext(ctx, `
			SELECT g.id, g.name, g.escrow FROM chat_groups g
			JOIN chat_group_members me ON me.group_id = g.id AND me.user_id = $1 AND NOT me.delivered
			WHERE g.escrow IS NOT NULL`, userID)
		return rowsOf(rows, err, func(r *sql.Rows) (g EscrowedGroup, err error) { err = r.Scan(&g.Group, &g.Name, &g.Escrow); return })
	}()
	return &out, err
}

// SetEscrow stores the server-sealed key of a group, if it has none yet and the sender is a member who has the key.
func SetEscrow(ctx context.Context, pool *sql.DB, userID int64, group int64, gid string, sealed []byte) (bool, error) {
	result, err := pool.ExecContext(ctx, `
		UPDATE chat_groups SET escrow = $1
		WHERE id = $2 AND gid = $3 AND escrow IS NULL
		  AND EXISTS (SELECT 1 FROM chat_group_members me WHERE me.group_id = $2 AND me.user_id = $4 AND me.delivered)`,
		sealed, group, gid, userID)
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows == 1, nil
}

// GroupGID returns a website group's chat id, or "" when it has none yet.
func GroupGID(ctx context.Context, pool *sql.DB, group int64) (string, error) {
	var gid sql.NullString
	err := pool.QueryRowContext(ctx, `SELECT gid FROM chat_groups WHERE id = $1`, group).Scan(&gid)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return gid.String, err
}

// SetGroupGID records the chat group id the owner's terminal made for a website group.
func SetGroupGID(ctx context.Context, pool *sql.DB, userID int64, group int64, gid string) (bool, error) {
	transaction, err := pool.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer transaction.Rollback()
	result, err := transaction.ExecContext(ctx, `UPDATE chat_groups SET gid = $1 WHERE id = $2 AND owner_id = $3 AND gid IS NULL`, gid, group, userID)
	if err != nil {
		return false, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return false, nil
	}
	_, err = transaction.ExecContext(ctx, `UPDATE chat_group_members SET delivered = true WHERE group_id = $1 AND user_id = $2`, group, userID)
	return err == nil, transaction.Commit()
}

// AddGrant stores a sealed key for a member who has not got it yet. It returns
// false when the sender is not a member who has the key, or the target is not waiting.
func AddGrant(ctx context.Context, pool *sql.DB, fromUser int64, group int64, targetHandle string, sealed []byte) (bool, error) {
	result, err := pool.ExecContext(ctx, `
		INSERT INTO chat_group_grants (group_id, user_id, from_user_id, sealed, created_at)
		SELECT m.group_id, m.user_id, $1, $4, (now() AT TIME ZONE 'UTC')
		FROM chat_group_members m
		JOIN accounts a ON a.user_id = m.user_id AND a.handle = $3
		WHERE m.group_id = $2 AND NOT m.delivered
		  AND EXISTS (SELECT 1 FROM chat_group_members me WHERE me.group_id = $2 AND me.user_id = $1 AND me.delivered)
		ON CONFLICT DO NOTHING`, fromUser, group, targetHandle, sealed)
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows == 1, nil
}

// AcceptGrant is called once the recipient's terminal has stored the key.
func AcceptGrant(ctx context.Context, pool *sql.DB, userID int64, group int64) error {
	transaction, err := pool.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	if _, err := transaction.ExecContext(ctx, `DELETE FROM chat_group_grants WHERE group_id = $1 AND user_id = $2`, group, userID); err != nil {
		return err
	}
	if _, err := transaction.ExecContext(ctx, `UPDATE chat_group_members SET delivered = true WHERE group_id = $1 AND user_id = $2`, group, userID); err != nil {
		return err
	}
	return transaction.Commit()
}

// WebGroupMember says whether a chat group id belongs to a website group and, if so, whether the
// user is still one of its members. Groups made only in a terminal are open to whoever has the key.
func WebGroupMember(ctx context.Context, pool *sql.DB, gid string, userID int64) (website bool, member bool, err error) {
	var groups, members int
	err = pool.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT g.id), COUNT(m.user_id)
		FROM chat_groups g
		LEFT JOIN chat_group_members m ON m.group_id = g.id AND m.user_id = $2
		WHERE g.gid = $1`, gid, userID).Scan(&groups, &members)
	return groups > 0, members > 0, err
}

// KickMember removes a member from a website group on the creator's say-so. It returns the
// handle's chat group id check results as one of the Err values above.
func KickMember(ctx context.Context, pool *sql.DB, ownerID int64, gid string, handle string) error {
	transaction, err := pool.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()

	var group, owner int64
	if err := transaction.QueryRowContext(ctx, `SELECT id, owner_id FROM chat_groups WHERE gid = $1`, gid).Scan(&group, &owner); errors.Is(err, sql.ErrNoRows) {
		return ErrNotWebGroup
	} else if err != nil {
		return err
	}
	if owner != ownerID {
		return ErrNotOwner
	}
	var target int64
	if err := transaction.QueryRowContext(ctx, `SELECT user_id FROM accounts WHERE handle = $1 AND user_id IS NOT NULL`, handle).Scan(&target); errors.Is(err, sql.ErrNoRows) {
		return ErrNotMember
	} else if err != nil {
		return err
	}
	if target == owner {
		return ErrKickOwner
	}
	result, err := transaction.ExecContext(ctx, `DELETE FROM chat_group_members WHERE group_id = $1 AND user_id = $2`, group, target)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrNotMember
	}
	for _, statement := range []string{
		`DELETE FROM chat_group_grants WHERE group_id = $1 AND user_id = $2`,
		`DELETE FROM chat_group_invites WHERE group_id = $1 AND invitee_id = $2`,
	} {
		if _, err := transaction.ExecContext(ctx, statement, group, target); err != nil {
			return err
		}
	}
	return transaction.Commit()
}

// Profile is what people can see about a user with /whois.
type Profile struct {
	Name   string
	Bio    string
	Joined int64 // unix seconds
}

// GetProfile returns nil when the handle has no website user behind it.
func GetProfile(ctx context.Context, pool *sql.DB, handle string) (*Profile, error) {
	var profile Profile
	var created sql.NullTime
	err := pool.QueryRowContext(ctx, `
		SELECT u.name, COALESCE(u.bio, ''), u.created_at
		FROM accounts a JOIN users u ON u.id = a.user_id WHERE a.handle = $1`, handle).Scan(&profile.Name, &profile.Bio, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if created.Valid {
		profile.Joined = created.Time.Unix()
	}
	return &profile, err
}

// SetBio saves a user's bio.
func SetBio(ctx context.Context, pool *sql.DB, userID int64, bio string) error {
	_, err := pool.ExecContext(ctx, `UPDATE users SET bio = NULLIF($1, ''), updated_at = (now() AT TIME ZONE 'UTC') WHERE id = $2`, bio, userID)
	return err
}

// SetPresence records that a person has the chat open (or just closed it). The website reads it to show
// who is online; it counts a person as online only while last_seen is recent, so a crash cannot leave
// anyone online for ever.
func SetPresence(ctx context.Context, pool *sql.DB, handle string, online bool) error {
	_, err := pool.ExecContext(ctx, `
		INSERT INTO presence (handle, online, last_seen) VALUES ($1, $2, extract(epoch FROM now())::bigint)
		ON CONFLICT (handle) DO UPDATE SET online = EXCLUDED.online, last_seen = EXCLUDED.last_seen`, handle, online)
	return err
}

// RecordShare notes that a member shared a file in a website group, for the group's page. It returns false
// when the chat is not a website group, or the person is not in it.
func RecordShare(ctx context.Context, pool *sql.DB, userID int64, gid string, name string, size int64, hash string, expires time.Time) (bool, error) {
	result, err := pool.ExecContext(ctx, `
		INSERT INTO chat_group_files (group_id, user_id, name, size, md5, expires_at, created_at)
		SELECT g.id, $1, $3, $4, NULLIF($5, ''), $6, (now() AT TIME ZONE 'UTC')
		FROM chat_groups g
		JOIN chat_group_members m ON m.group_id = g.id AND m.user_id = $1
		WHERE g.gid = $2`, userID, gid, name, size, hash, expires.UTC())
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows == 1, nil
}
