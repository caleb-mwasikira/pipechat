package database

import (
	"context"
	"database/sql"
	"errors"
)

// Release is one client program the admin uploaded on the website (the binaries table).
type Release struct {
	ID       int64
	OS       string // linux, windows or macos
	Arch     string // amd64 or arm64
	Version  string
	Filename string
	Path     string // under the website's storage/app/private folder
	Size     int64
	MD5      string
	SHA256   string
}

const releaseColumns = `id, os, arch, version, filename, path, size, md5, sha256`

func scanRelease(row interface{ Scan(...any) error }) (Release, error) {
	var r Release
	err := row.Scan(&r.ID, &r.OS, &r.Arch, &r.Version, &r.Filename, &r.Path, &r.Size, &r.MD5, &r.SHA256)
	return r, err
}

// ListReleases returns every version uploaded for a system and processor.
func ListReleases(ctx context.Context, pool *sql.DB, osName string, arch string) ([]Release, error) {
	rows, err := pool.QueryContext(ctx, `SELECT `+releaseColumns+` FROM binaries WHERE os = $1 AND arch = $2`, osName, arch)
	return rowsOf(rows, err, func(r *sql.Rows) (Release, error) { return scanRelease(r) })
}

// GetRelease returns nil when there is no such upload.
func GetRelease(ctx context.Context, pool *sql.DB, id int64) (*Release, error) {
	r, err := scanRelease(pool.QueryRowContext(ctx, `SELECT `+releaseColumns+` FROM binaries WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &r, err
}
