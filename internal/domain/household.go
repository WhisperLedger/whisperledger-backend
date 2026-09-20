package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type HouseholdMemberRole string

const (
	MemberRoleAdmin  HouseholdMemberRole = "admin"
	MemberRoleMember HouseholdMemberRole = "member"
)

type Household struct {
	ID         uuid.UUID         `json:"id"`
	Name       string            `json:"name"`
	InviteCode string            `json:"invite_code"`
	Currency   string            `json:"currency"`
	CreatedBy  uuid.UUID         `json:"created_by"`
	Members    []HouseholdMember `json:"members,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

type HouseholdMember struct {
	ID          uuid.UUID           `json:"id"`
	HouseholdID uuid.UUID           `json:"household_id"`
	UserID      uuid.UUID           `json:"user_id"`
	User        *User               `json:"user,omitempty"`
	Role        HouseholdMemberRole `json:"role"`
	JoinedAt    time.Time           `json:"joined_at"`
}

type DebtResolution struct {
	FromUserID   uuid.UUID `json:"from_user_id"`
	FromUserName string    `json:"from_user_name"`
	ToUserID     uuid.UUID `json:"to_user_id"`
	ToUserName   string    `json:"to_user_name"`
	Amount       float64   `json:"amount"`
}

type HouseholdBalance struct {
	UserID    uuid.UUID `json:"user_id"`
	UserName  string    `json:"user_name"`
	NetOwed   float64   `json:"net_owed"` // Positive = others owe them, Negative = they owe others
}

type HouseholdRepository interface {
	Create(ctx context.Context, household *Household) error
	GetByID(ctx context.Context, id uuid.UUID) (*Household, error)
	GetByInviteCode(ctx context.Context, code string) (*Household, error)
	AddMember(ctx context.Context, member *HouseholdMember) error
	RemoveMember(ctx context.Context, householdID, userID uuid.UUID) error
	ListMembers(ctx context.Context, householdID uuid.UUID) ([]HouseholdMember, error)
	GetHouseholdBalances(ctx context.Context, householdID uuid.UUID) ([]HouseholdBalance, error)
	RecordSettlement(ctx context.Context, householdID, fromUser, toUser uuid.UUID, amount float64) error
}
