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

type ExpenseRepository struct {
	pool *pgxpool.Pool
}

func NewExpenseRepository(pool *pgxpool.Pool) *ExpenseRepository {
	return &ExpenseRepository{pool: pool}
}

func (r *ExpenseRepository) Create(ctx context.Context, e *domain.Expense) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	now := time.Now().UTC()
	e.CreatedAt = now
	e.UpdatedAt = now

	expenseQuery := `
		INSERT INTO expenses (id, user_id, household_id, outflow_type, amount, personal_share, recoverable_amount, category, merchant, payment_method, note, date, is_shared, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`
	_, err = tx.Exec(ctx, expenseQuery,
		e.ID, e.UserID, e.HouseholdID, e.OutflowType, e.Amount, e.PersonalShare, e.RecoverableAmount,
		e.Category, e.Merchant, e.PaymentMethod, e.Note, e.Date, e.IsShared, e.CreatedAt, e.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert expense: %w", err)
	}

	if len(e.Splits) > 0 {
		splitQuery := `
			INSERT INTO expense_splits (id, expense_id, user_id, amount, is_settled, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`
		for i := range e.Splits {
			if e.Splits[i].ID == uuid.Nil {
				e.Splits[i].ID = uuid.New()
			}
			e.Splits[i].ExpenseID = e.ID
			e.Splits[i].CreatedAt = now
			_, err = tx.Exec(ctx, splitQuery,
				e.Splits[i].ID, e.Splits[i].ExpenseID, e.Splits[i].UserID, e.Splits[i].Amount, e.Splits[i].IsSettled, e.Splits[i].CreatedAt,
			)
			if err != nil {
				return fmt.Errorf("failed to insert split: %w", err)
			}
		}
	}

	return tx.Commit(ctx)
}

func (r *ExpenseRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Expense, error) {
	query := `
		SELECT id, user_id, household_id, outflow_type, amount, personal_share, recoverable_amount, category, merchant, payment_method, note, date, is_shared, created_at, updated_at
		FROM expenses
		WHERE id = $1
	`
	var e domain.Expense
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&e.ID, &e.UserID, &e.HouseholdID, &e.OutflowType, &e.Amount, &e.PersonalShare, &e.RecoverableAmount,
		&e.Category, &e.Merchant, &e.PaymentMethod, &e.Note, &e.Date, &e.IsShared, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	// Fetch splits if shared
	if e.IsShared {
		splitsQuery := `
			SELECT id, expense_id, user_id, amount, is_settled, created_at
			FROM expense_splits
			WHERE expense_id = $1
		`
		rows, err := r.pool.Query(ctx, splitsQuery, id)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var s domain.ExpenseSplit
				if err := rows.Scan(&s.ID, &s.ExpenseID, &s.UserID, &s.Amount, &s.IsSettled, &s.CreatedAt); err == nil {
					e.Splits = append(e.Splits, s)
				}
			}
		}
	}

	return &e, nil
}

func (r *ExpenseRepository) List(ctx context.Context, filter domain.ExpenseFilter) ([]*domain.Expense, int, error) {
	var conditions []string
	var args []interface{}
	idx := 1

	conditions = append(conditions, fmt.Sprintf("user_id = $%d", idx))
	args = append(args, filter.UserID)
	idx++

	if filter.HouseholdID != nil {
		conditions = append(conditions, fmt.Sprintf("household_id = $%d", idx))
		args = append(args, *filter.HouseholdID)
		idx++
	}

	if filter.OutflowType != nil {
		conditions = append(conditions, fmt.Sprintf("outflow_type = $%d", idx))
		args = append(args, string(*filter.OutflowType))
		idx++
	}

	if filter.Category != nil {
		conditions = append(conditions, fmt.Sprintf("category = $%d", idx))
		args = append(args, string(*filter.Category))
		idx++
	}

	if filter.StartDate != nil {
		conditions = append(conditions, fmt.Sprintf("date >= $%d", idx))
		args = append(args, *filter.StartDate)
		idx++
	}

	if filter.EndDate != nil {
		conditions = append(conditions, fmt.Sprintf("date <= $%d", idx))
		args = append(args, *filter.EndDate)
		idx++
	}

	whereClause := strings.Join(conditions, " AND ")
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM expenses WHERE %s", whereClause)
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
		SELECT id, user_id, household_id, outflow_type, amount, personal_share, recoverable_amount, category, merchant, payment_method, note, date, is_shared, created_at, updated_at
		FROM expenses
		WHERE %s
		ORDER BY date DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, idx, idx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var expenses []*domain.Expense
	for rows.Next() {
		var e domain.Expense
		if err := rows.Scan(
			&e.ID, &e.UserID, &e.HouseholdID, &e.OutflowType, &e.Amount, &e.PersonalShare, &e.RecoverableAmount,
			&e.Category, &e.Merchant, &e.PaymentMethod, &e.Note, &e.Date, &e.IsShared, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		expenses = append(expenses, &e)
	}

	return expenses, total, nil
}

func (r *ExpenseRepository) Update(ctx context.Context, e *domain.Expense) error {
	query := `
		UPDATE expenses
		SET amount = $1, personal_share = $2, recoverable_amount = $3, category = $4, merchant = $5, payment_method = $6, note = $7, date = $8, outflow_type = $9, is_shared = $10, updated_at = $11
		WHERE id = $12 AND user_id = $13
	`
	e.UpdatedAt = time.Now().UTC()
	_, err := r.pool.Exec(ctx, query,
		e.Amount, e.PersonalShare, e.RecoverableAmount, e.Category, e.Merchant, e.PaymentMethod, e.Note, e.Date, e.OutflowType, e.IsShared, e.UpdatedAt, e.ID, e.UserID,
	)
	return err
}

func (r *ExpenseRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM expenses WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	return err
}

func (r *ExpenseRepository) GetSummary(ctx context.Context, userID uuid.UUID, startDate, endDate time.Time) (*domain.OutflowSummary, error) {
	query := `
		SELECT 
			COALESCE(SUM(amount), 0),
			COALESCE(SUM(personal_share), 0),
			COALESCE(SUM(CASE WHEN outflow_type = 'shared_household' THEN personal_share ELSE 0 END), 0),
			COALESCE(SUM(recoverable_amount), 0)
		FROM expenses
		WHERE user_id = $1 AND date >= $2 AND date <= $3
	`
	summary := &domain.OutflowSummary{
		CategoryBreakdown: make(map[string]float64),
	}

	err := r.pool.QueryRow(ctx, query, userID, startDate, endDate).Scan(
		&summary.TotalOutflow,
		&summary.TruePersonalTotal,
		&summary.HouseholdShareTotal,
		&summary.RecoverableTotal,
	)
	if err != nil {
		return nil, err
	}

	catQuery := `
		SELECT category, COALESCE(SUM(personal_share), 0)
		FROM expenses
		WHERE user_id = $1 AND date >= $2 AND date <= $3
		GROUP BY category
	`
	rows, err := r.pool.Query(ctx, catQuery, userID, startDate, endDate)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cat string
			var amount float64
			if err := rows.Scan(&cat, &amount); err == nil {
				summary.CategoryBreakdown[cat] = amount
			}
		}
	}

	return summary, nil
}
