package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"whisperledger-backend/internal/domain"
)

var (
	ErrExpenseNotFound = errors.New("expense not found")
	ErrInvalidAmount   = errors.New("expense amount must be greater than zero")
)

type CreateExpenseInput struct {
	HouseholdID   *uuid.UUID             `json:"household_id,omitempty"`
	OutflowType   domain.OutflowType     `json:"outflow_type"`
	Amount        float64                `json:"amount"`
	Category      domain.ExpenseCategory `json:"category"`
	Merchant      string                 `json:"merchant"`
	PaymentMethod string                 `json:"payment_method"`
	Note          *string                `json:"note,omitempty"`
	Date          time.Time              `json:"date"`
	Splits        []SplitInput           `json:"splits,omitempty"`
}

type SplitInput struct {
	UserID uuid.UUID `json:"user_id"`
	Amount float64   `json:"amount"`
}

type LedgerService struct {
	expenseRepo   domain.ExpenseRepository
	householdRepo domain.HouseholdRepository
}

func NewLedgerService(expenseRepo domain.ExpenseRepository, householdRepo domain.HouseholdRepository) *LedgerService {
	return &LedgerService{
		expenseRepo:   expenseRepo,
		householdRepo: householdRepo,
	}
}

func (s *LedgerService) CreateExpense(ctx context.Context, userID uuid.UUID, input CreateExpenseInput) (*domain.Expense, error) {
	if input.Amount <= 0 {
		return nil, ErrInvalidAmount
	}

	if input.Date.IsZero() {
		input.Date = time.Now().UTC()
	}

	expense := &domain.Expense{
		ID:            uuid.New(),
		UserID:        userID,
		HouseholdID:   input.HouseholdID,
		OutflowType:   input.OutflowType,
		Amount:        input.Amount,
		Category:      input.Category,
		Merchant:      input.Merchant,
		PaymentMethod: input.PaymentMethod,
		Note:          input.Note,
		Date:          input.Date,
	}

	switch input.OutflowType {
	case domain.OutflowTruePersonal:
		expense.PersonalShare = input.Amount
		expense.RecoverableAmount = 0
		expense.IsShared = false

	case domain.OutflowRecoverable:
		expense.PersonalShare = 0
		expense.RecoverableAmount = input.Amount
		expense.IsShared = false

	case domain.OutflowSharedHousehold:
		expense.IsShared = true
		if len(input.Splits) > 0 {
			var myShare float64
			for _, sp := range input.Splits {
				expense.Splits = append(expense.Splits, domain.ExpenseSplit{
					ExpenseID: expense.ID,
					UserID:    sp.UserID,
					Amount:    sp.Amount,
					IsSettled: sp.UserID == userID,
				})
				if sp.UserID == userID {
					myShare += sp.Amount
				}
			}
			expense.PersonalShare = myShare
			expense.RecoverableAmount = input.Amount - myShare
		} else if input.HouseholdID != nil {
			// Auto-split equally among household members
			members, err := s.householdRepo.ListMembers(ctx, *input.HouseholdID)
			if err == nil && len(members) > 0 {
				splitEach := input.Amount / float64(len(members))
				for _, m := range members {
					expense.Splits = append(expense.Splits, domain.ExpenseSplit{
						ExpenseID: expense.ID,
						UserID:    m.UserID,
						Amount:    splitEach,
						IsSettled: m.UserID == userID,
					})
				}
				expense.PersonalShare = splitEach
				expense.RecoverableAmount = input.Amount - splitEach
			} else {
				expense.PersonalShare = input.Amount
				expense.RecoverableAmount = 0
			}
		} else {
			expense.PersonalShare = input.Amount
			expense.RecoverableAmount = 0
		}

	default:
		expense.OutflowType = domain.OutflowTruePersonal
		expense.PersonalShare = input.Amount
		expense.RecoverableAmount = 0
		expense.IsShared = false
	}

	if err := s.expenseRepo.Create(ctx, expense); err != nil {
		return nil, err
	}

	return expense, nil
}

func (s *LedgerService) GetExpense(ctx context.Context, id uuid.UUID) (*domain.Expense, error) {
	expense, err := s.expenseRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if expense == nil {
		return nil, ErrExpenseNotFound
	}
	return expense, nil
}

func (s *LedgerService) ListExpenses(ctx context.Context, filter domain.ExpenseFilter) ([]*domain.Expense, int, error) {
	return s.expenseRepo.List(ctx, filter)
}

func (s *LedgerService) DeleteExpense(ctx context.Context, id uuid.UUID) error {
	return s.expenseRepo.Delete(ctx, id)
}

func (s *LedgerService) GetSummary(ctx context.Context, userID uuid.UUID, startDate, endDate time.Time) (*domain.OutflowSummary, error) {
	if startDate.IsZero() {
		// Default to current month start
		now := time.Now().UTC()
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	if endDate.IsZero() {
		endDate = time.Now().UTC()
	}

	return s.expenseRepo.GetSummary(ctx, userID, startDate, endDate)
}
