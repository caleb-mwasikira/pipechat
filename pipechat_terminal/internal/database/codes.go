package database

import (
	"context"
	"database/sql"
	"errors"
)

// Invite codes: short codes a website group's creator makes on the website. Redeeming one adds the
// person to the group; the key then reaches them through the normal sync (the server holds an escrow).

// ErrNoSuchCode means the code is unknown, expired, revoked or used up. They look the same on purpose.
var ErrNoSuchCode = errors.New("no such invite code")

// ErrTooManyGroups means the person is in as many website groups as the website allows.
var ErrTooManyGroups = errors.New("too many groups")

const webGroupsPerUser = 50

// JoinByCode puts a person in the group a code belongs to. code must be upper case without a dash.
// It returns the group's name and whether the person was newly added; a code is only used up by
// someone who was not in the group already.
func JoinByCode(ctx context.Context, pool *sql.DB, userID int64, code string) (name string, added bool, err error) {
	tx, err := pool.BeginTx(ctx, nil)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()

	var codeID, groupID int64
	err = tx.QueryRowContext(ctx, `
		SELECT c.id, g.id, g.name FROM chat_group_invite_codes c JOIN chat_groups g ON g.id = c.group_id
		WHERE c.code = $1 AND c.expires_at > (now() AT TIME ZONE 'UTC') AND c.uses < c.max_uses FOR UPDATE OF c`, code).
		Scan(&codeID, &groupID, &name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, ErrNoSuchCode
	}
	if err != nil {
		return "", false, err
	}

	var already bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM chat_group_members WHERE group_id = $1 AND user_id = $2)`, groupID, userID).Scan(&already); err != nil {
		return "", false, err
	}
	if already {
		return name, false, tx.Commit()
	}

	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM chat_group_members WHERE user_id = $1`, userID).Scan(&count); err != nil {
		return "", false, err
	}
	if count >= webGroupsPerUser {
		return "", false, ErrTooManyGroups
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO chat_group_members (group_id, user_id, created_at, updated_at) VALUES ($1, $2, now() AT TIME ZONE 'UTC', now() AT TIME ZONE 'UTC')`, groupID, userID); err != nil {
		return "", false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE chat_group_invite_codes SET uses = uses + 1, updated_at = now() AT TIME ZONE 'UTC' WHERE id = $1`, codeID); err != nil {
		return "", false, err
	}
	return name, true, tx.Commit()
}
