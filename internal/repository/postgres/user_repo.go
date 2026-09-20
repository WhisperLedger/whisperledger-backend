package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"whisperledger-backend/internal/domain"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	query := `
		INSERT INTO users (id, email, password_hash, full_name, phone_number, avatar_url, role, is_active, household_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	now := time.Now().UTC()
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	u.CreatedAt = now
	u.UpdatedAt = now

	_, err := r.pool.Exec(ctx, query,
		u.ID, u.Email, u.PasswordHash, u.FullName, u.PhoneNumber, u.AvatarURL,
		u.Role, u.IsActive, u.HouseholdID, u.CreatedAt, u.UpdatedAt,
	)
	return err
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, full_name, phone_number, avatar_url, role, is_active, household_id, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	var u domain.User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.PhoneNumber, &u.AvatarURL,
		&u.Role, &u.IsActive, &u.HouseholdID, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, full_name, phone_number, avatar_url, role, is_active, household_id, created_at, updated_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`
	var u domain.User
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.PhoneNumber, &u.AvatarURL,
		&u.Role, &u.IsActive, &u.HouseholdID, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) Update(ctx context.Context, u *domain.User) error {
	query := `
		UPDATE users
		SET full_name = $1, phone_number = $2, avatar_url = $3, role = $4, is_active = $5, household_id = $6, updated_at = $7
		WHERE id = $8
	`
	u.UpdatedAt = time.Now().UTC()
	_, err := r.pool.Exec(ctx, query,
		u.FullName, u.PhoneNumber, u.AvatarURL, u.Role, u.IsActive, u.HouseholdID, u.UpdatedAt, u.ID,
	)
	return err
}

func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]*domain.User, int, error) {
	countQuery := `SELECT COUNT(*) FROM users`
	var total int
	if err := r.pool.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, email, full_name, phone_number, avatar_url, role, is_active, household_id, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(
			&u.ID, &u.Email, &u.FullName, &u.PhoneNumber, &u.AvatarURL,
			&u.Role, &u.IsActive, &u.HouseholdID, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		users = append(users, &u)
	}

	return users, total, nil
}

func (r *UserRepository) SetHousehold(ctx context.Context, userID uuid.UUID, householdID *uuid.UUID) error {
	query := `UPDATE users SET household_id = $1, updated_at = $2 WHERE id = $3`
	_, err := r.pool.Exec(ctx, query, householdID, time.Now().UTC(), userID)
	return err
}
