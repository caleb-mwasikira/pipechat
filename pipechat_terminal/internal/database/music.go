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
