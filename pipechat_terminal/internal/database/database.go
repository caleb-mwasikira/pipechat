// Package database stores everything the server must remember in PostgreSQL:
// accounts (each linked to a website user), groups (ids and hashed join tokens), ciphertext history and file
// records. It never holds a key or a plain text message. The tables live in the
// same database as the Laravel website's tables (users, sessions, ...).
package database

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const schema = `
CREATE TABLE IF NOT EXISTS accounts (
	handle     TEXT PRIMARY KEY,
	auth_hash  TEXT NOT NULL,
	pubkey     TEXT NOT NULL,
	vault      BYTEA NOT NULL,
	created_at BIGINT NOT NULL
);
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS user_id BIGINT;
CREATE UNIQUE INDEX IF NOT EXISTS accounts_user_id_key ON accounts (user_id);
CREATE TABLE IF NOT EXISTS inbox (
	handle     TEXT NOT NULL REFERENCES accounts(handle),
	peer       TEXT NOT NULL,
	created_at BIGINT NOT NULL,
	PRIMARY KEY (handle, peer)
);
CREATE TABLE IF NOT EXISTS groups (
	id         TEXT PRIMARY KEY,
	token_hash BYTEA NOT NULL,
	last_seq   BIGINT NOT NULL DEFAULT 0,
	created_at BIGINT NOT NULL
);
CREATE TABLE IF NOT EXISTS messages (
	group_id TEXT NOT NULL REFERENCES groups(id),
	seq      BIGINT NOT NULL,
	ts       BIGINT NOT NULL,
	ct       BYTEA NOT NULL,
	PRIMARY KEY (group_id, seq)
);
CREATE TABLE IF NOT EXISTS presence (
	handle    TEXT PRIMARY KEY,
	online    BOOLEAN NOT NULL,
	last_seen BIGINT NOT NULL
);
CREATE TABLE IF NOT EXISTS open_groups (
	name_key   TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	gid        TEXT NOT NULL UNIQUE,
	code       TEXT NOT NULL,
	creator    TEXT NOT NULL,
	created_at BIGINT NOT NULL
);
CREATE TABLE IF NOT EXISTS counters (
	name  TEXT PRIMARY KEY,
	value BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS files (
	id         TEXT PRIMARY KEY,
	handle     TEXT NOT NULL,
	size       BIGINT NOT NULL,
	created_at BIGINT NOT NULL
);`

// Lifetime totals. Messages and files are deleted as they age out, so the
// admin dashboard cannot count them by looking at the tables.
const (
	CounterMessages = "messages_sent"
	CounterFiles    = "files_sent"
)

// bump adds one to a lifetime counter, using q (a pool or a transaction).
func bump(ctx context.Context, q interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, name string) error {
	_, err := q.ExecContext(ctx,
		`INSERT INTO counters (name, value) VALUES ($1, 1) ON CONFLICT (name) DO UPDATE SET value = counters.value + 1`, name)
	return err
}

// Connect opens the PostgreSQL database at url (for example
// postgres://user:password@localhost:5432/pipechat_website) and makes sure the tables exist.
func Connect(ctx context.Context, url string) (*sql.DB, error) {
	pool, err := sql.Open("pgx", url)
	if err != nil {
		return nil, err
	}
	if err := pool.PingContext(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if _, err := pool.ExecContext(ctx, schema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("creating tables: %w", err)
	}
	return pool, nil
}
