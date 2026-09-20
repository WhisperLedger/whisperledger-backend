package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"whisperledger-backend/internal/domain"
)

type ReceivableRepository struct {
	pool *pgxpool.Pool
}

func NewReceivableRepository(pool *pgxpool.Pool) *ReceivableRepository {
	return &ReceivableRepository{pool: pool}
}

func (r *ReceivableRepository) Create(ctx context.Context, rec *domain.Receivable) error {
	if rec.ID == uuid.Nil {
		rec.ID = uuid.New()
	}
	now := time.Now().UTC()
	rec.CreatedAt = now
	rec.UpdatedAt = now
	if rec.Status == "" {
		rec.Status = domain.ReceivablePending
	}

	query := `
		INSERT INTO receivables (id, user_id, title, debtor_name, debtor_phone, amount, settled_amount, type, status, due_date, source_expense_id, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	_, err := r.pool.Exec(ctx, query,
		rec.ID, rec.UserID, rec.Title, rec.DebtorName, rec.DebtorPhone, rec.Amount,
		rec.SettledAmount, rec.Type, rec.Status, rec.DueDate, rec.SourceExpenseID, rec.Notes,
		rec.CreatedAt, rec.UpdatedAt,
	)
	return err
}

func (r *ReceivableRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Receivable, error) {
	query := `
		SELECT id, user_id, title, debtor_name, debtor_phone, amount, settled_amount, type, status, due_date, source_expense_id, notes, settled_at, created_at, updated_at
		FROM receivables
		WHERE id = $1
	`
	var rec domain.Receivable
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&rec.ID, &rec.UserID, &rec.Title, &rec.DebtorName, &rec.DebtorPhone, &rec.Amount,
		&rec.SettledAmount, &rec.Type, &rec.Status, &rec.DueDate, &rec.SourceExpenseID, &rec.Notes,
		&rec.SettledAt, &rec.CreatedAt, &rec.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &rec, nil
}

func (r *ReceivableRepository) List(ctx context.Context, filter domain.ReceivableFilter) ([]*domain.Receivable, int, error) {
	var conditions []string
	var args []interface{}
	idx := 1

	conditions = append(conditions, fmt.Sprintf("user_id = $%d", idx))
	args = append(args, filter.UserID)
	idx++

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", idx))
		args = append(args, string(*filter.Status))
		idx++
	}

	if filter.Type != nil {
		conditions = append(conditions, fmt.Sprintf("type = $%d", idx))
		args = append(args, string(*filter.Type))
		idx++
	}

	whereClause := strings.Join(conditions, " AND ")
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM receivables WHERE %s", whereClause)
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, title, debtor_name, debtor_phone, amount, settled_amount, type, status, due_date, source_expense_id, notes, settled_at, created_at, updated_at
		FROM receivables
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, idx, idx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []*domain.Receivable
	for rows.Next() {
		var rec domain.Receivable
		if err := rows.Scan(
			&rec.ID, &rec.UserID, &rec.Title, &rec.DebtorName, &rec.DebtorPhone, &rec.Amount,
			&rec.SettledAmount, &rec.Type, &rec.Status, &rec.DueDate, &rec.SourceExpenseID, &rec.Notes,
			&rec.SettledAt, &rec.CreatedAt, &rec.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, &rec)
	}

	return list, total, nil
}

func (r *ReceivableRepository) Update(ctx context.Context, rec *domain.Receivable) error {
	query := `
		UPDATE receivables
		SET title = $1, debtor_name = $2, debtor_phone = $3, amount = $4, settled_amount = $5, type = $6, status = $7, due_date = $8, notes = $9, settled_at = $10, updated_at = $11
		WHERE id = $12 AND user_id = $13
	`
	rec.UpdatedAt = time.Now().UTC()
	_, err := r.pool.Exec(ctx, query,
		rec.Title, rec.DebtorName, rec.DebtorPhone, rec.Amount, rec.SettledAmount, rec.Type,
		rec.Status, rec.DueDate, rec.Notes, rec.SettledAt, rec.UpdatedAt, rec.ID, rec.UserID,
	)
	return err
}

func (r *ReceivableRepository) Settle(ctx context.Context, id uuid.UUID, amount float64) error {
	rec, err := r.GetByID(ctx, id)
	if err != nil || rec == nil {
		return fmt.Errorf("receivable not found")
	}

	rec.SettledAmount += amount
	now := time.Now().UTC()
	if rec.SettledAmount >= rec.Amount {
		rec.Status = domain.ReceivableSettled
		rec.SettledAt = &now
	} else {
		rec.Status = domain.ReceivablePartial
	}

	query := `
		UPDATE receivables
		SET settled_amount = $1, status = $2, settled_at = $3, updated_at = $4
		WHERE id = $5
	`
	_, err = r.pool.Exec(ctx, query, rec.SettledAmount, rec.Status, rec.SettledAt, now, id)
	return err
}

func (r *ReceivableRepository) GetTotalPending(ctx context.Context, userID uuid.UUID) (float64, error) {
	query := `
		SELECT COALESCE(SUM(amount - settled_amount), 0)
		FROM receivables
		WHERE user_id = $1 AND status != 'settled' AND status != 'cancelled'
	`
	var total float64
	err := r.pool.QueryRow(ctx, query, userID).Scan(&total)
	return total, err
}
