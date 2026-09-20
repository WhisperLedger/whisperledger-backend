package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type OutflowType string

const (
	OutflowTruePersonal   OutflowType = "true_personal"
	OutflowSharedHousehold OutflowType = "shared_household"
	OutflowRecoverable     OutflowType = "recoverable"
)

type ExpenseCategory string

const (
	CategoryGroceries      ExpenseCategory = "groceries"
	CategoryRentBills      ExpenseCategory = "rent_bills"
	CategoryDiningOut      ExpenseCategory = "dining_out"
	CategorySubscriptions  ExpenseCategory = "subscriptions"
	CategoryTransportation ExpenseCategory = "transportation"
	CategoryShopping       ExpenseCategory = "shopping"
	CategoryHealthcare     ExpenseCategory = "healthcare"
	CategoryTravel         ExpenseCategory = "travel"
	CategoryEntertainment  ExpenseCategory = "entertainment"
	CategoryOther          ExpenseCategory = "other"
)

type ExpenseSplit struct {
	ID        uuid.UUID `json:"id"`
	ExpenseID uuid.UUID `json:"expense_id"`
	UserID    uuid.UUID `json:"user_id"`
	Amount    float64   `json:"amount"`
	IsSettled bool      `json:"is_settled"`
	CreatedAt time.Time `json:"created_at"`
}

type Expense struct {
	ID                uuid.UUID       `json:"id"`
	UserID            uuid.UUID       `json:"user_id"`
	HouseholdID       *uuid.UUID      `json:"household_id,omitempty"`
	OutflowType       OutflowType     `json:"outflow_type"`
	Amount            float64         `json:"amount"`             // Total money paid out
	PersonalShare     float64         `json:"personal_share"`     // True cost to current user
	RecoverableAmount float64         `json:"recoverable_amount"` // Expected back from others/vendors
	Category          ExpenseCategory `json:"category"`
	Merchant          string          `json:"merchant"`
	PaymentMethod     string          `json:"payment_method"`
	Note              *string         `json:"note,omitempty"`
	Date              time.Time       `json:"date"`
	IsShared          bool            `json:"is_shared"`
	Splits            []ExpenseSplit  `json:"splits,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type OutflowSummary struct {
	TotalOutflow       float64            `json:"total_outflow"`
	TruePersonalTotal  float64            `json:"true_personal_total"`
	HouseholdShareTotal float64           `json:"household_share_total"`
	RecoverableTotal   float64            `json:"recoverable_total"`
	CategoryBreakdown  map[string]float64 `json:"category_breakdown"`
	LeakAlertCount     int                `json:"leak_alert_count"`
}

type ExpenseFilter struct {
	UserID      uuid.UUID
	HouseholdID *uuid.UUID
	OutflowType *OutflowType
	Category    *ExpenseCategory
	StartDate   *time.Time
	EndDate     *time.Time
	Limit       int
	Offset      int
}

type ExpenseRepository interface {
	Create(ctx context.Context, expense *Expense) error
	GetByID(ctx context.Context, id uuid.UUID) (*Expense, error)
	List(ctx context.Context, filter ExpenseFilter) ([]*Expense, int, error)
	Update(ctx context.Context, expense *Expense) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetSummary(ctx context.Context, userID uuid.UUID, startDate, endDate time.Time) (*OutflowSummary, error)
}
