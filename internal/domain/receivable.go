package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ReceivableType string

const (
	ReceivableFriendLend    ReceivableType = "friend_lend"
	ReceivableVendorRefund  ReceivableType = "vendor_refund"
	ReceivableSecurityDep   ReceivableType = "security_deposit"
	ReceivableGroupAdvance  ReceivableType = "group_advance"
	ReceivableOther         ReceivableType = "other"
)

type ReceivableStatus string

const (
	ReceivablePending   ReceivableStatus = "pending"
	ReceivablePartial   ReceivableStatus = "partially_settled"
	ReceivableSettled   ReceivableStatus = "settled"
	ReceivableOverdue   ReceivableStatus = "overdue"
	ReceivableCancelled ReceivableStatus = "cancelled"
)

type Receivable struct {
	ID              uuid.UUID        `json:"id"`
	UserID          uuid.UUID        `json:"user_id"`
	Title           string           `json:"title"`
	DebtorName      string           `json:"debtor_name"`
	DebtorPhone     *string          `json:"debtor_phone,omitempty"`
	Amount          float64          `json:"amount"`
	SettledAmount   float64          `json:"settled_amount"`
	Type            ReceivableType   `json:"type"`
	Status          ReceivableStatus `json:"status"`
	DueDate         *time.Time       `json:"due_date,omitempty"`
	SourceExpenseID *uuid.UUID       `json:"source_expense_id,omitempty"`
	Notes           *string          `json:"notes,omitempty"`
	SettledAt       *time.Time       `json:"settled_at,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

type ReceivableFilter struct {
	UserID uuid.UUID
	Status *ReceivableStatus
	Type   *ReceivableType
	Limit  int
	Offset int
}

type ReceivableRepository interface {
	Create(ctx context.Context, r *Receivable) error
	GetByID(ctx context.Context, id uuid.UUID) (*Receivable, error)
	List(ctx context.Context, filter ReceivableFilter) ([]*Receivable, int, error)
	Update(ctx context.Context, r *Receivable) error
	Settle(ctx context.Context, id uuid.UUID, amount float64) error
	GetTotalPending(ctx context.Context, userID uuid.UUID) (float64, error)
}
