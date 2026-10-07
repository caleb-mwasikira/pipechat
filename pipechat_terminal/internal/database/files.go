package database

import (
	"context"
	"database/sql"
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
