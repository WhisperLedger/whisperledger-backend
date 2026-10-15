package postgres

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type InterestRepository struct {
	pool *pgxpool.Pool
}

func NewInterestRepository(pool *pgxpool.Pool) *InterestRepository {
	return &InterestRepository{pool: pool}
}

// Subscribe is intentionally idempotent. The public endpoint returns the same
// result for new and existing addresses so it cannot be used to enumerate them.
func (r *InterestRepository) Subscribe(ctx context.Context, email string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO interest_signups (email, source, consented_at)
		VALUES ($1, 'website', NOW())
		ON CONFLICT (email) DO UPDATE
		SET consented_at = EXCLUDED.consented_at,
			unsubscribed_at = NULL,
			updated_at = NOW()
	`, strings.ToLower(strings.TrimSpace(email)))
	return err
}
