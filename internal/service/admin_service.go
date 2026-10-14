package service

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"whisperledger-backend/internal/domain"
)

type AdminService struct {
	pool      *pgxpool.Pool
	userRepo  domain.UserRepository
	startTime time.Time
}

func NewAdminService(pool *pgxpool.Pool, userRepo domain.UserRepository) *AdminService {
	return &AdminService{
		pool:      pool,
		userRepo:  userRepo,
		startTime: time.Now().UTC(),
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

func (s *AdminService) GetCloudTelemetry(ctx context.Context) (*domain.CloudTelemetry, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	poolStat := s.pool.Stat()
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "production"
	}

	revision := os.Getenv("K_REVISION")
	if revision == "" {
		revision = "whisperledger-api-00002-d24"
	}

	serviceName := os.Getenv("K_SERVICE")
	if serviceName == "" {
		serviceName = "whisperledger-api"
	}

	telemetry := &domain.CloudTelemetry{
		Service:        serviceName,
		Region:         "asia-south1",
		Revision:       revision,
		Environment:    env,
		Status:         "healthy",
		UptimeSeconds:  int64(time.Since(s.startTime).Seconds()),
		GoVersion:      runtime.Version(),
		MemoryAllocMB:  float64(m.Alloc) / 1024 / 1024,
		MemorySysMB:    float64(m.Sys) / 1024 / 1024,
		DBPoolTotal:    poolStat.TotalConns(),
		DBPoolIdle:     poolStat.IdleConns(),
		DBPoolAcquired: poolStat.AcquiredConns(),
		DailyVolumes:   make([]domain.DailyVolumeStat, 0),
		RecentLogs:     make([]domain.SystemLogEntry, 0),
		GCPConsoleLinks: map[string]string{
			"cloud_run": "https://console.cloud.google.com/run/detail/asia-south1/whisperledger-api/metrics?project=whisperledger-94715",
			"cloud_logging": "https://console.cloud.google.com/logs/query?project=whisperledger-94715",
			"cloud_build": "https://console.cloud.google.com/cloud-build/builds?project=whisperledger-94715",
			"neon_console": "https://console.neon.tech/",
		},
	}

	// Fetch 14-day spending volume trend
	rows, err := s.pool.Query(ctx, `
		SELECT TO_CHAR(date, 'YYYY-MM-DD') AS day, COALESCE(SUM(amount), 0), COUNT(*)
		FROM expenses
		WHERE date >= NOW() - INTERVAL '14 days'
		GROUP BY day
		ORDER BY day ASC
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var stat domain.DailyVolumeStat
			if err := rows.Scan(&stat.Date, &stat.Amount, &stat.Count); err == nil {
				telemetry.DailyVolumes = append(telemetry.DailyVolumes, stat)
			}
		}
	}

	// Fetch recent operational events from DB (latest expenses & users)
	eventRows, err := s.pool.Query(ctx, `
		SELECT 'expense' AS source, created_at, 'INFO' AS level, 
		       'Expense logged: ₹' || amount || ' at ' || merchant AS message
		FROM expenses
		ORDER BY created_at DESC
		LIMIT 10
	`)
	if err == nil {
		defer eventRows.Close()
		for eventRows.Next() {
			var entry domain.SystemLogEntry
			if err := eventRows.Scan(&entry.Source, &entry.Timestamp, &entry.Level, &entry.Message); err == nil {
				telemetry.RecentLogs = append(telemetry.RecentLogs, entry)
			}
		}
	}

	// Add service boot log
	telemetry.RecentLogs = append(telemetry.RecentLogs, domain.SystemLogEntry{
		Timestamp: s.startTime,
		Level:     "INFO",
		Message:   fmt.Sprintf("Cloud Run revision %s initialized and connected to Neon PostgreSQL pool", revision),
		Source:    "system",
	})

	return telemetry, nil
}

func (s *AdminService) ListUsers(ctx context.Context, limit, offset int) ([]*domain.User, int, error) {
	return s.userRepo.List(ctx, limit, offset)
}

