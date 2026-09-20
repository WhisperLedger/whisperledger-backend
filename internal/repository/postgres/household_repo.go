package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"whisperledger-backend/internal/domain"
)

type HouseholdRepository struct {
	pool *pgxpool.Pool
}

func NewHouseholdRepository(pool *pgxpool.Pool) *HouseholdRepository {
	return &HouseholdRepository{pool: pool}
}

func (r *HouseholdRepository) Create(ctx context.Context, h *domain.Household) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if h.ID == uuid.Nil {
		h.ID = uuid.New()
	}
	now := time.Now().UTC()
	h.CreatedAt = now
	h.UpdatedAt = now

	query := `
		INSERT INTO households (id, name, invite_code, currency, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = tx.Exec(ctx, query, h.ID, h.Name, h.InviteCode, h.Currency, h.CreatedBy, h.CreatedAt, h.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create household: %w", err)
	}

	// Add creator as household admin
	memberQuery := `
		INSERT INTO household_members (id, household_id, user_id, role, joined_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err = tx.Exec(ctx, memberQuery, uuid.New(), h.ID, h.CreatedBy, domain.MemberRoleAdmin, now)
	if err != nil {
		return fmt.Errorf("failed to add creator as member: %w", err)
	}

	// Update creator's household_id
	userUpdateQuery := `UPDATE users SET household_id = $1, updated_at = $2 WHERE id = $3`
	_, err = tx.Exec(ctx, userUpdateQuery, h.ID, now, h.CreatedBy)
	if err != nil {
		return fmt.Errorf("failed to update user household_id: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *HouseholdRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Household, error) {
	query := `
		SELECT id, name, invite_code, currency, created_by, created_at, updated_at
		FROM households
		WHERE id = $1
	`
	var h domain.Household
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&h.ID, &h.Name, &h.InviteCode, &h.Currency, &h.CreatedBy, &h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	members, err := r.ListMembers(ctx, id)
	if err == nil {
		h.Members = members
	}

	return &h, nil
}

func (r *HouseholdRepository) GetByInviteCode(ctx context.Context, code string) (*domain.Household, error) {
	query := `
		SELECT id, name, invite_code, currency, created_by, created_at, updated_at
		FROM households
		WHERE UPPER(invite_code) = UPPER($1)
	`
	var h domain.Household
	err := r.pool.QueryRow(ctx, query, code).Scan(
		&h.ID, &h.Name, &h.InviteCode, &h.Currency, &h.CreatedBy, &h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	members, err := r.ListMembers(ctx, h.ID)
	if err == nil {
		h.Members = members
	}

	return &h, nil
}

func (r *HouseholdRepository) AddMember(ctx context.Context, member *domain.HouseholdMember) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if member.ID == uuid.Nil {
		member.ID = uuid.New()
	}
	member.JoinedAt = time.Now().UTC()

	query := `
		INSERT INTO household_members (id, household_id, user_id, role, joined_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (household_id, user_id) DO NOTHING
	`
	_, err = tx.Exec(ctx, query, member.ID, member.HouseholdID, member.UserID, member.Role, member.JoinedAt)
	if err != nil {
		return err
	}

	userQuery := `UPDATE users SET household_id = $1, updated_at = $2 WHERE id = $3`
	_, err = tx.Exec(ctx, userQuery, member.HouseholdID, time.Now().UTC(), member.UserID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *HouseholdRepository) RemoveMember(ctx context.Context, householdID, userID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `DELETE FROM household_members WHERE household_id = $1 AND user_id = $2`
	_, err = tx.Exec(ctx, query, householdID, userID)
	if err != nil {
		return err
	}

	userQuery := `UPDATE users SET household_id = NULL, updated_at = $1 WHERE id = $2`
	_, err = tx.Exec(ctx, userQuery, time.Now().UTC(), userID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *HouseholdRepository) ListMembers(ctx context.Context, householdID uuid.UUID) ([]domain.HouseholdMember, error) {
	query := `
		SELECT hm.id, hm.household_id, hm.user_id, hm.role, hm.joined_at,
		       u.email, u.full_name, u.avatar_url
		FROM household_members hm
		JOIN users u ON hm.user_id = u.id
		WHERE hm.household_id = $1
		ORDER BY hm.joined_at ASC
	`
	rows, err := r.pool.Query(ctx, query, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []domain.HouseholdMember
	for rows.Next() {
		var m domain.HouseholdMember
		var email, fullName string
		var avatarURL *string
		if err := rows.Scan(
			&m.ID, &m.HouseholdID, &m.UserID, &m.Role, &m.JoinedAt,
			&email, &fullName, &avatarURL,
		); err != nil {
			return nil, err
		}
		m.User = &domain.User{
			ID:        m.UserID,
			Email:     email,
			FullName:  fullName,
			AvatarURL: avatarURL,
		}
		members = append(members, m)
	}

	return members, nil
}

func (r *HouseholdRepository) GetHouseholdBalances(ctx context.Context, householdID uuid.UUID) ([]domain.HouseholdBalance, error) {
	// Net calculation:
	// A user's net balance = (Total money they paid for shared household expenses)
	//                      - (Total split portions assigned to them across all shared expenses)
	//                      + (Settlements received)
	//                      - (Settlements paid)
	query := `
		WITH members AS (
			SELECT hm.user_id, u.full_name
			FROM household_members hm
			JOIN users u ON hm.user_id = u.id
			WHERE hm.household_id = $1
		),
		paid AS (
			SELECT user_id, COALESCE(SUM(amount), 0) AS total_paid
			FROM expenses
			WHERE household_id = $1 AND is_shared = true
			GROUP BY user_id
		),
		owed AS (
			SELECT es.user_id, COALESCE(SUM(es.amount), 0) AS total_owed
			FROM expense_splits es
			JOIN expenses e ON es.expense_id = e.id
			WHERE e.household_id = $1
			GROUP BY es.user_id
		),
		settled_paid AS (
			SELECT from_user_id AS user_id, COALESCE(SUM(amount), 0) AS total_settled_paid
			FROM settlements
			WHERE household_id = $1
			GROUP BY from_user_id
		),
		settled_received AS (
			SELECT to_user_id AS user_id, COALESCE(SUM(amount), 0) AS total_settled_received
			FROM settlements
			WHERE household_id = $1
			GROUP BY to_user_id
		)
		SELECT 
			m.user_id,
			m.full_name,
			(COALESCE(p.total_paid, 0) - COALESCE(o.total_owed, 0) + COALESCE(sp.total_settled_paid, 0) - COALESCE(sr.total_settled_received, 0)) AS net_owed
		FROM members m
		LEFT JOIN paid p ON m.user_id = p.user_id
		LEFT JOIN owed o ON m.user_id = o.user_id
		LEFT JOIN settled_paid sp ON m.user_id = sp.user_id
		LEFT JOIN settled_received sr ON m.user_id = sr.user_id
	`
	rows, err := r.pool.Query(ctx, query, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var balances []domain.HouseholdBalance
	for rows.Next() {
		var b domain.HouseholdBalance
		if err := rows.Scan(&b.UserID, &b.UserName, &b.NetOwed); err != nil {
			return nil, err
		}
		balances = append(balances, b)
	}

	return balances, nil
}

func (r *HouseholdRepository) RecordSettlement(ctx context.Context, householdID, fromUser, toUser uuid.UUID, amount float64) error {
	query := `
		INSERT INTO settlements (id, household_id, from_user_id, to_user_id, amount, settled_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.pool.Exec(ctx, query, uuid.New(), householdID, fromUser, toUser, amount, time.Now().UTC())
	return err
}
