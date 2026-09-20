package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleUser           Role = "user"
	RoleHouseholdAdmin Role = "household_admin"
	RolePlatformAdmin  Role = "platform_admin"
)

type User struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	FullName     string     `json:"full_name"`
	PhoneNumber  *string    `json:"phone_number,omitempty"`
	AvatarURL    *string    `json:"avatar_url,omitempty"`
	Role         Role       `json:"role"`
	IsActive     bool       `json:"is_active"`
	HouseholdID  *uuid.UUID `json:"household_id,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) error
	List(ctx context.Context, limit, offset int) ([]*User, int, error)
	SetHousehold(ctx context.Context, userID uuid.UUID, householdID *uuid.UUID) error
}
