package database

import (
	"context"
	"database/sql"
	"errors"
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

type Sync struct {
	Create   []NewGroup `json:"create"`   // groups this user owns that still need creating
	Grant    []ToGrant  `json:"grant"`    // members of this user's groups waiting for the key
	Incoming []Incoming `json:"incoming"` // keys sent to this user
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
	return &out, err
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
		SELECT m.group_id, m.user_id, $1, $4, now()
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
