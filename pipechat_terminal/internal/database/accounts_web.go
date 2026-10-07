package database

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Accounts made and repaired from the terminal. They are the website's own rows (users, password_reset_tokens),
// written the way Laravel writes them, so the website and the terminal share one account and one password.
// Times are in UTC, as Laravel keeps them.

var (
	ErrEmailTaken  = errors.New("that email already has an account")
	ErrHandleTaken = errors.New("that handle is taken")
)

const utcNow = "(now() AT TIME ZONE 'UTC')"

// CreateWebUser adds a user. passwordHash is a bcrypt hash.
func CreateWebUser(ctx context.Context, pool *sql.DB, name string, handle string, email string, passwordHash string) error {
	transaction, err := pool.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	var taken bool
	if err := transaction.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE lower(email) = lower($1))`, email).Scan(&taken); err != nil {
		return err
	} else if taken {
		return ErrEmailTaken
	}
	if err := transaction.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE handle = $1)`, handle).Scan(&taken); err != nil {
		return err
	} else if taken {
		return ErrHandleTaken
	}
	if _, err := transaction.ExecContext(ctx,
		`INSERT INTO users (name, handle, email, password, created_at, updated_at) VALUES ($1, $2, $3, $4, `+utcNow+`, `+utcNow+`)`,
		name, handle, email, passwordHash); err != nil {
		// Two people registering at once: the unique index says which one lost.
		if strings.Contains(err.Error(), "users_email_unique") {
			return ErrEmailTaken
		}
		if strings.Contains(err.Error(), "users_handle_unique") {
			return ErrHandleTaken
		}
		return err
	}
	return transaction.Commit()
}

// StoreResetToken keeps the hash of a password reset code for an email, replacing an earlier one. It returns false,
// and changes nothing, when a code was made less than minGap ago (the website has the same one-a-minute rule).
func StoreResetToken(ctx context.Context, pool *sql.DB, email string, tokenHash string, minGap time.Duration) (bool, error) {
	var age sql.NullFloat64
	err := pool.QueryRowContext(ctx, `SELECT EXTRACT(EPOCH FROM (`+utcNow+` - created_at)) FROM password_reset_tokens WHERE email = $1`, email).Scan(&age)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	if age.Valid && time.Duration(age.Float64*float64(time.Second)) < minGap {
		return false, nil
	}
	_, err = pool.ExecContext(ctx, `
		INSERT INTO password_reset_tokens (email, token, created_at) VALUES ($1, $2, `+utcNow+`)
		ON CONFLICT (email) DO UPDATE SET token = EXCLUDED.token, created_at = EXCLUDED.created_at`, email, tokenHash)
	return err == nil, err
}

// GetResetToken returns the stored hash and how old it is; found is false when there is none.
func GetResetToken(ctx context.Context, pool *sql.DB, email string) (hash string, age time.Duration, found bool, err error) {
	var seconds float64
	err = pool.QueryRowContext(ctx, `SELECT token, EXTRACT(EPOCH FROM (`+utcNow+` - created_at)) FROM password_reset_tokens WHERE email = $1`, email).Scan(&hash, &seconds)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, false, nil
	}
	return hash, time.Duration(seconds * float64(time.Second)), err == nil, err
}

// ResetPassword sets a new password, uses up the code, and signs the user out of the website everywhere.
func ResetPassword(ctx context.Context, pool *sql.DB, email string, passwordHash string) error {
	transaction, err := pool.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	var userID int64
	if err := transaction.QueryRowContext(ctx,
		`UPDATE users SET password = $1, remember_token = NULL, updated_at = `+utcNow+` WHERE lower(email) = lower($2) RETURNING id`,
		passwordHash, email).Scan(&userID); err != nil {
		return err
	}
	if _, err := transaction.ExecContext(ctx, `DELETE FROM password_reset_tokens WHERE email = $1`, email); err != nil {
		return err
	}
	if err := transaction.Commit(); err != nil {
		return err
	}
	// Anyone who had the old password is logged out of the website. (Sessions live in this table.)
	pool.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	return nil
}
