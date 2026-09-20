package service

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"whisperledger-backend/internal/domain"
)

type AdminService struct {
	pool     *pgxpool.Pool
	userRepo domain.UserRepository
}

func NewAdminService(pool *pgxpool.Pool, userRepo domain.UserRepository) *AdminService {
	return &AdminService{
		pool:     pool,
		userRepo: userRepo,
	}
}

func (s *AdminService) GetPlatformMetrics(ctx context.Context) (*domain.AdminPlatformMetrics, error) {
	metrics := &domain.AdminPlatformMetrics{
		GeneratedAt: time.Now().UTC(),
	}

	// 1. Total users
	_ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&metrics.TotalUsers)

	// 2. Total households
	_ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM households`).Scan(&metrics.TotalHouseholds)

	// 3. Total expenses and volume
	_ = s.pool.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(SUM(amount), 0) 
		FROM expenses
	`).Scan(&metrics.TotalExpensesTracked, &metrics.TotalVolumeTracked)

	// 4. Total pending receivables
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount - settled_amount), 0)
		FROM receivables
		WHERE status != 'settled' AND status != 'cancelled'
	`).Scan(&metrics.TotalReceivablesPending)

	// 5. Active users in past 7 days
	sevenDaysAgo := time.Now().UTC().AddDate(0, 0, -7)
	_ = s.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT user_id) 
		FROM expenses 
		WHERE created_at >= $1
	`, sevenDaysAgo).Scan(&metrics.ActiveUsers7Days)

	return metrics, nil
}

func (s *AdminService) ListUsers(ctx context.Context, limit, offset int) ([]*domain.User, int, error) {
	return s.userRepo.List(ctx, limit, offset)
}
