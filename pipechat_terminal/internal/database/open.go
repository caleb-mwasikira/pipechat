package database

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Open (public) groups. Anyone can join one by its name, so the server keeps the group's invite
// code and hands it to whoever asks. They are not private: the server can read them, and so can
// every person who joins. Private groups never come here.

// OpenGroup is a public group's listing.
type OpenGroup struct {
	Name    string `json:"name"`
	Creator string `json:"creator"`
	Code    string `json:"code,omitempty"` // only when asking for one group, to join it
}

// AddOpenGroup lists a group under a name. It returns false when the name or the group is already listed.
func AddOpenGroup(ctx context.Context, pool *sql.DB, name string, gid string, code string, creator string) (bool, error) {
	result, err := pool.ExecContext(ctx,
		`INSERT INTO open_groups (name_key, name, gid, code, creator, created_at) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT DO NOTHING`,
		strings.ToLower(name), name, gid, code, creator, time.Now().Unix())
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows == 1, nil
}

// GetOpenGroup returns nil when no public group has that name.
func GetOpenGroup(ctx context.Context, pool *sql.DB, name string) (*OpenGroup, error) {
	var group OpenGroup
	err := pool.QueryRowContext(ctx, `SELECT name, creator, code FROM open_groups WHERE name_key = $1`, strings.ToLower(name)).
		Scan(&group.Name, &group.Creator, &group.Code)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &group, err
}

// ListOpenGroups returns the newest public groups, without their codes.
func ListOpenGroups(ctx context.Context, pool *sql.DB, limit int) ([]OpenGroup, error) {
	rows, err := pool.QueryContext(ctx, `SELECT name, creator FROM open_groups ORDER BY created_at DESC, name_key LIMIT $1`, limit)
	return rowsOf(rows, err, func(r *sql.Rows) (g OpenGroup, err error) { err = r.Scan(&g.Name, &g.Creator); return })
}

// RemoveOpenGroup takes a group off the list, if the person asking made it. The group itself stays.
func RemoveOpenGroup(ctx context.Context, pool *sql.DB, name string, creator string) (bool, error) {
	result, err := pool.ExecContext(ctx, `DELETE FROM open_groups WHERE name_key = $1 AND creator = $2`, strings.ToLower(name), creator)
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows == 1, nil
}
