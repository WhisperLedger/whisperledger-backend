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
	Plan         string     `json:"plan"`
	Status       string     `json:"status"`
	PushReady    bool       `json:"push_ready"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type BroadcastNotification struct {
	ID             uuid.UUID  `json:"id"`
	AppName        string     `json:"app_name"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	ImageURL       string     `json:"image_url"`
	Target         string     `json:"target"`
	TargetUserID   *uuid.UUID `json:"target_user_id,omitempty"`
	Type           string     `json:"type"`
	DeliveredCount int        `json:"delivered_count"`
	DispatchStatus string     `json:"dispatch_status"`
	SentAt         time.Time  `json:"sent_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) error
	UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error
	UpdateSubscription(ctx context.Context, id uuid.UUID, plan, status string) error
	List(ctx context.Context, limit, offset int) ([]*User, int, error)
	SetHousehold(ctx context.Context, userID uuid.UUID, householdID *uuid.UUID) error
}
