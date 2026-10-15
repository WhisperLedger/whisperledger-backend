package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
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
			"cloud_run":     "https://console.cloud.google.com/run/detail/asia-south1/whisperledger-api/metrics?project=whisperledger-94715",
			"cloud_logging": "https://console.cloud.google.com/logs/query?project=whisperledger-94715",
			"cloud_build":   "https://console.cloud.google.com/cloud-build/builds?project=whisperledger-94715",
			"neon_console":  "https://console.neon.tech/",
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

func (s *AdminService) UpdateUserSubscription(ctx context.Context, id uuid.UUID, plan, status string) error {
	return s.userRepo.UpdateSubscription(ctx, id, plan, status)
}

func (s *AdminService) RecordAudit(ctx context.Context, actorID uuid.UUID, action, targetType, targetID string, metadata map[string]interface{}) {
	payload, err := json.Marshal(metadata)
	if err != nil {
		return
	}
	// Audit writes are intentionally best-effort: an operations action must not
	// be repeated if a secondary audit insert has a transient failure.
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO audit_events (actor_user_id, action, target_type, target_id, metadata)
		VALUES ($1, $2, $3, $4, $5)
	`, actorID, action, targetType, targetID, payload)
}

type expoPushMessage struct {
	To        string                 `json:"to"`
	Title     string                 `json:"title"`
	Body      string                 `json:"body"`
	Sound     string                 `json:"sound"`
	Priority  string                 `json:"priority"`
	ChannelID string                 `json:"channelId"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

func (s *AdminService) dispatchExpoPush(ctx context.Context, tokens []string, title, body, notifType string, notifID uuid.UUID) int {
	if len(tokens) == 0 {
		return 0
	}

	var messages []expoPushMessage
	for _, t := range tokens {
		clean := strings.TrimSpace(t)
		if clean == "" {
			continue
		}
		messages = append(messages, expoPushMessage{
			To:        clean,
			Title:     title,
			Body:      body,
			Sound:     "default",
			Priority:  "high",
			ChannelID: "default",
			Data: map[string]interface{}{
				"type":            notifType,
				"notification_id": notifID.String(),
			},
		})
	}

	if len(messages) == 0 {
		return 0
	}

	payload, err := json.Marshal(messages)
	if err != nil {
		return 0
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://exp.host/--/api/v2/push/send", bytes.NewBuffer(payload))
	if err != nil {
		return 0
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return len(messages)
	}
	return 0
}

func (s *AdminService) CreateBroadcastNotification(ctx context.Context, notif *domain.BroadcastNotification) error {
	if notif.ID == uuid.Nil {
		notif.ID = uuid.New()
	}
	if notif.AppName == "" {
		notif.AppName = "WhisperLedger"
	}
	notif.SentAt = time.Now().UTC()
	notif.CreatedAt = time.Now().UTC()

	// Query target device push tokens from PostgreSQL
	var tokens []string
	if notif.Target == "single_user" && notif.TargetUserID != nil {
		var tok *string
		err := s.pool.QueryRow(ctx, `SELECT push_token FROM users WHERE id = $1 AND push_token IS NOT NULL AND push_token != ''`, notif.TargetUserID).Scan(&tok)
		if err == nil && tok != nil && *tok != "" {
			tokens = append(tokens, *tok)
		}
	} else if notif.Target == "pro_only" {
		rows, err := s.pool.Query(ctx, `SELECT push_token FROM users WHERE plan = 'pro' AND push_token IS NOT NULL AND push_token != ''`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var tok string
				if err := rows.Scan(&tok); err == nil && tok != "" {
					tokens = append(tokens, tok)
				}
			}
		}
	} else {
		// Target 'all'
		rows, err := s.pool.Query(ctx, `SELECT push_token FROM users WHERE push_token IS NOT NULL AND push_token != ''`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var tok string
				if err := rows.Scan(&tok); err == nil && tok != "" {
					tokens = append(tokens, tok)
				}
			}
		}
	}

	if len(tokens) > 0 {
		delivered := s.dispatchExpoPush(ctx, tokens, notif.Title, notif.Body, notif.Type, notif.ID)
		notif.DeliveredCount = delivered
		notif.DispatchStatus = "delivered"
	} else {
		notif.DeliveredCount = 0
		notif.DispatchStatus = "delivered"
	}

	query := `
		INSERT INTO broadcast_history (id, app_name, title, body, image_url, target, target_user_id, type, delivered_count, dispatch_status, sent_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := s.pool.Exec(ctx, query,
		notif.ID, notif.AppName, notif.Title, notif.Body, notif.ImageURL,
		notif.Target, notif.TargetUserID, notif.Type, notif.DeliveredCount,
		notif.DispatchStatus, notif.SentAt, notif.CreatedAt,
	)
	return err
}

func (s *AdminService) ListBroadcastNotifications(ctx context.Context, limit int) ([]*domain.BroadcastNotification, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	query := `
		SELECT id, app_name, title, body, COALESCE(image_url, ''), target, target_user_id, type, delivered_count, dispatch_status, sent_at, created_at
		FROM broadcast_history
		ORDER BY sent_at DESC
		LIMIT $1
	`
	rows, err := s.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.BroadcastNotification
	for rows.Next() {
		var b domain.BroadcastNotification
		if err := rows.Scan(
			&b.ID, &b.AppName, &b.Title, &b.Body, &b.ImageURL, &b.Target,
			&b.TargetUserID, &b.Type, &b.DeliveredCount, &b.DispatchStatus,
			&b.SentAt, &b.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, &b)
	}
	return list, nil
}
