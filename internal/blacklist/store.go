package blacklist

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// IsBlocked returns true if the user is currently blacklisted.
func (s *Store) IsBlocked(ctx context.Context, userID int64) (bool, error) {
	var blocked bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
            SELECT 1 FROM bot_blacklist
            WHERE user_id = $1
              AND (blocked_until IS NULL OR blocked_until > NOW())
        )`,
		userID,
	).Scan(&blocked)
	return blocked, err
}

// Block adds or updates a blacklist entry for the given user.
func (s *Store) Block(ctx context.Context, userID int64, reason string, blockedBy int64, blockedUntil *time.Time) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO bot_blacklist (user_id, reason, blocked_by, blocked_until)
         VALUES ($1, $2, $3, $4)
         ON CONFLICT (user_id) DO UPDATE
           SET reason = EXCLUDED.reason,
               blocked_by = EXCLUDED.blocked_by,
               blocked_until = EXCLUDED.blocked_until,
               created_at = NOW()`,
		userID, reason, blockedBy, blockedUntil,
	)
	return err
}

// Unblock removes the blacklist entry for the given user.
func (s *Store) Unblock(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM bot_blacklist WHERE user_id = $1`, userID)
	return err
}
