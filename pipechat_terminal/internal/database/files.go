package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func AddFile(ctx context.Context, pool *sql.DB, id string, handle string, size int64) error {
	if _, err := pool.ExecContext(ctx, `INSERT INTO files (id, handle, size, created_at) VALUES ($1, $2, $3, $4)`, id, handle, size, time.Now().Unix()); err != nil {
		return err
	}
	return bump(ctx, pool, CounterFiles)
}

// ExpiredFiles lists the ids of files uploaded before cutoff.
func ExpiredFiles(ctx context.Context, pool *sql.DB, cutoff time.Time) ([]string, error) {
	rows, err := pool.QueryContext(ctx, `SELECT id FROM files WHERE created_at < $1`, cutoff.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func DeleteFile(ctx context.Context, pool *sql.DB, id string) error {
	_, err := pool.ExecContext(ctx, `DELETE FROM files WHERE id = $1`, id)
	return err
}

// Why a delete was refused.
var (
	ErrNoSuchFile = errors.New("no such file")
	ErrNotAllowed = errors.New("only the person who uploaded it, or the group's owner, can delete this")
)

// DeleteSharedFile removes a stored file's record, and the website's listing of it, if the person asking
// uploaded it or owns the website group it was shared in. The caller then removes the bytes from disk.
func DeleteSharedFile(ctx context.Context, pool *sql.DB, userID int64, handle string, id string) error {
	var uploader string
	known := true
	if err := pool.QueryRowContext(ctx, `SELECT handle FROM files WHERE id = $1`, id).Scan(&uploader); errors.Is(err, sql.ErrNoRows) {
		known = false
	} else if err != nil {
		return err
	}
	allowed := known && uploader == handle
	var owner bool
	if !allowed {
		if err := pool.QueryRowContext(ctx, `
			SELECT EXISTS (SELECT 1 FROM chat_group_files f JOIN chat_groups g ON g.id = f.group_id WHERE f.file_id = $1 AND g.owner_id = $2)`,
			id, userID).Scan(&owner); err != nil {
			return err
		}
		allowed = owner
	}
	switch {
	case !allowed && !known:
		return ErrNoSuchFile // the same answer whether it exists or not, so ids cannot be probed
	case !allowed:
		return ErrNotAllowed
	}
	transaction, err := pool.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	if _, err := transaction.ExecContext(ctx, `DELETE FROM chat_group_files WHERE file_id = $1`, id); err != nil {
		return err
	}
	if _, err := transaction.ExecContext(ctx, `DELETE FROM files WHERE id = $1`, id); err != nil {
		return err
	}
	return transaction.Commit()
}
