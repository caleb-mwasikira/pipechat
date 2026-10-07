package database

import (
	"context"
	"database/sql"
	"errors"
)

// Track is one song in the library the website keeps (the tracks table).
type Track struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Ext      string `json:"ext"`
	Size     int64  `json:"size"`
	Uploader string `json:"uploader"`
	MD5      string `json:"md5"`
	Path     string `json:"-"` // under the website's storage/app/private folder
}

const trackColumns = `t.id, t.title, COALESCE(t.artist, ''), t.extension, t.size, COALESCE(u.handle, ''), COALESCE(t.md5, ''), t.path`

func scanTrack(row interface{ Scan(...any) error }) (Track, error) {
	var t Track
	err := row.Scan(&t.ID, &t.Title, &t.Artist, &t.Ext, &t.Size, &t.Uploader, &t.MD5, &t.Path)
	return t, err
}

// ListTracks returns the newest tracks first.
func ListTracks(ctx context.Context, pool *sql.DB, limit int) ([]Track, error) {
	rows, err := pool.QueryContext(ctx,
		`SELECT `+trackColumns+` FROM tracks t LEFT JOIN users u ON u.id = t.uploaded_by ORDER BY t.id DESC LIMIT $1`, limit)
	return rowsOf(rows, err, func(r *sql.Rows) (Track, error) { return scanTrack(r) })
}

// GetTrack returns nil when there is no such track.
func GetTrack(ctx context.Context, pool *sql.DB, id int64) (*Track, error) {
	t, err := scanTrack(pool.QueryRowContext(ctx,
		`SELECT `+trackColumns+` FROM tracks t LEFT JOIN users u ON u.id = t.uploaded_by WHERE t.id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &t, err
}

// DeleteTrack removes a song from the library if the person asking uploaded it or is an administrator.
// It returns the stored file's path when no other track uses it, for the caller to remove from disk.
func DeleteTrack(ctx context.Context, pool *sql.DB, userID int64, id int64) (path string, removeFile bool, err error) {
	var uploader sql.NullInt64
	if err := pool.QueryRowContext(ctx, `SELECT path, uploaded_by FROM tracks WHERE id = $1`, id).Scan(&path, &uploader); errors.Is(err, sql.ErrNoRows) {
		return "", false, ErrNoSuchFile
	} else if err != nil {
		return "", false, err
	}
	var admin bool
	if err := pool.QueryRowContext(ctx, `SELECT COALESCE(is_admin, false) FROM users WHERE id = $1`, userID).Scan(&admin); err != nil {
		return "", false, err
	}
	if !admin && !(uploader.Valid && uploader.Int64 == userID) {
		return "", false, ErrNotAllowed
	}
	transaction, err := pool.BeginTx(ctx, nil)
	if err != nil {
		return "", false, err
	}
	defer transaction.Rollback()
	if _, err := transaction.ExecContext(ctx, `DELETE FROM tracks WHERE id = $1`, id); err != nil {
		return "", false, err
	}
	var others int
	if err := transaction.QueryRowContext(ctx, `SELECT count(*) FROM tracks WHERE path = $1`, path).Scan(&others); err != nil {
		return "", false, err
	}
	return path, others == 0, transaction.Commit()
}
