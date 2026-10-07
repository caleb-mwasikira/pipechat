package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Account struct {
	Handle   string
	AuthHash string // hex SHA-256 of the auth key
	PubKey   string
	Vault    []byte
}

// CreateAccount returns false when the handle is taken.
func CreateAccount(ctx context.Context, pool *sql.DB, account Account) (bool, error) {
	if account.Vault == nil {
		account.Vault = []byte{} // the column cannot be NULL
	}
	result, err := pool.ExecContext(ctx,
		`INSERT INTO accounts (handle, auth_hash, pubkey, vault, created_at) VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`,
		account.Handle, account.AuthHash, account.PubKey, account.Vault, time.Now().Unix())
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows == 1, nil
}

// GetAccount returns nil when there is no such handle.
func GetAccount(ctx context.Context, pool *sql.DB, handle string) (*Account, error) {
	account := &Account{Handle: handle}
	err := pool.QueryRowContext(ctx, `SELECT auth_hash, pubkey, vault FROM accounts WHERE handle = $1`, handle).
		Scan(&account.AuthHash, &account.PubKey, &account.Vault)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return account, err
}

func CountAccounts(ctx context.Context, pool *sql.DB) (int, error) {
	var count int
	err := pool.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts`).Scan(&count)
	return count, err
}

func SetVault(ctx context.Context, pool *sql.DB, handle string, vault []byte) error {
	_, err := pool.ExecContext(ctx, `UPDATE accounts SET vault = $1 WHERE handle = $2`, vault, handle)
	return err
}

// AddInbox records that peer wants a direct chat with handle. The newest limit entries are kept.
func AddInbox(ctx context.Context, pool *sql.DB, handle string, peer string, limit int) error {
	if _, err := pool.ExecContext(ctx,
		`INSERT INTO inbox (handle, peer, created_at) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, handle, peer, time.Now().Unix()); err != nil {
		return err
	}
	_, err := pool.ExecContext(ctx,
		`DELETE FROM inbox WHERE handle = $1 AND peer NOT IN (SELECT peer FROM inbox WHERE handle = $2 ORDER BY created_at DESC LIMIT $3)`,
		handle, handle, limit)
	return err
}

func ClearInbox(ctx context.Context, pool *sql.DB, handle string, peer string) error {
	_, err := pool.ExecContext(ctx, `DELETE FROM inbox WHERE handle = $1 AND peer = $2`, handle, peer)
	return err
}

func GetInbox(ctx context.Context, pool *sql.DB, handle string) ([]string, error) {
	rows, err := pool.QueryContext(ctx, `SELECT peer FROM inbox WHERE handle = $1 ORDER BY created_at`, handle)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var peers []string
	for rows.Next() {
		var peer string
		if err := rows.Scan(&peer); err != nil {
			return nil, err
		}
		peers = append(peers, peer)
	}
	return peers, rows.Err()
}
