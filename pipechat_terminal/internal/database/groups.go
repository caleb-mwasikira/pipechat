package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Message is one stored ciphertext.
type Message struct {
	Seq uint64
	TS  int64 // unix millis
	CT  []byte
}

// CreateGroup returns false when the id is taken.
func CreateGroup(ctx context.Context, pool *sql.DB, id string, tokenHash []byte) (bool, error) {
	result, err := pool.ExecContext(ctx,
		`INSERT INTO groups (id, token_hash, created_at) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, id, tokenHash, time.Now().Unix())
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows == 1, nil
}

// GetGroupTokenHash returns nil when there is no such group.
func GetGroupTokenHash(ctx context.Context, pool *sql.DB, id string) ([]byte, error) {
	var hash []byte
	err := pool.QueryRowContext(ctx, `SELECT token_hash FROM groups WHERE id = $1`, id).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return hash, err
}

func CountGroups(ctx context.Context, pool *sql.DB) (int, error) {
	var count int
	err := pool.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups`).Scan(&count)
	return count, err
}

// AppendMessage stores a message with the next sequence number and keeps only the newest keep messages.
// Sequence numbers are time based, so they keep increasing even if the database is ever restored.
func AppendMessage(ctx context.Context, pool *sql.DB, groupID string, ciphertext []byte, keep int) (Message, error) {
	transaction, err := pool.BeginTx(ctx, nil)
	if err != nil {
		return Message{}, err
	}
	defer transaction.Rollback()

	now := time.Now()
	message := Message{TS: now.UnixMilli(), CT: ciphertext}
	var last int64
	if err := transaction.QueryRowContext(ctx, `SELECT last_seq FROM groups WHERE id = $1 FOR UPDATE`, groupID).Scan(&last); err != nil {
		return Message{}, err
	}
	message.Seq = max(uint64(last)+1, uint64(now.UnixNano()))

	if _, err := transaction.ExecContext(ctx, `UPDATE groups SET last_seq = $1 WHERE id = $2`, int64(message.Seq), groupID); err != nil {
		return Message{}, err
	}
	if _, err := transaction.ExecContext(ctx,
		`INSERT INTO messages (group_id, seq, ts, ct) VALUES ($1, $2, $3, $4)`, groupID, int64(message.Seq), message.TS, ciphertext); err != nil {
		return Message{}, err
	}
	if _, err := transaction.ExecContext(ctx,
		`DELETE FROM messages WHERE group_id = $1 AND seq < (SELECT MIN(seq) FROM (SELECT seq FROM messages WHERE group_id = $2 ORDER BY seq DESC LIMIT $3) AS newest)`,
		groupID, groupID, keep); err != nil {
		return Message{}, err
	}
	if err := bump(ctx, transaction, CounterMessages); err != nil {
		return Message{}, err
	}
	return message, transaction.Commit()
}

// MessagesSince returns up to limit of the newest messages with a sequence number above since, oldest first.
func MessagesSince(ctx context.Context, pool *sql.DB, groupID string, since uint64, limit int) ([]Message, error) {
	rows, err := pool.QueryContext(ctx,
		`SELECT seq, ts, ct FROM (SELECT seq, ts, ct FROM messages WHERE group_id = $1 AND seq > $2 ORDER BY seq DESC LIMIT $3) AS newest ORDER BY seq`,
		groupID, int64(since), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var messages []Message
	for rows.Next() {
		var message Message
		var seq int64
		if err := rows.Scan(&seq, &message.TS, &message.CT); err != nil {
			return nil, err
		}
		message.Seq = uint64(seq)
		messages = append(messages, message)
	}
	return messages, rows.Err()
}
