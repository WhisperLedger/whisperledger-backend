package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
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
	Location      *string                `json:"location,omitempty"`
	Latitude      *float64               `json:"latitude,omitempty"`
	Longitude     *float64               `json:"longitude,omitempty"`
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
	userRepo      domain.UserRepository
}

func NewLedgerService(expenseRepo domain.ExpenseRepository, householdRepo domain.HouseholdRepository, userRepo domain.UserRepository) *LedgerService {
	return &LedgerService{
		expenseRepo:   expenseRepo,
		householdRepo: householdRepo,
		userRepo:      userRepo,
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
		Location:      input.Location,
		Latitude:      input.Latitude,
		Longitude:     input.Longitude,
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

	if expense.IsShared && expense.HouseholdID != nil {
		go s.notifyHouseholdSpend(*expense.HouseholdID, userID, expense)
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

// GetUserExpense prevents an authenticated user from discovering another
// person's ledger entry by UUID.
func (s *LedgerService) GetUserExpense(ctx context.Context, userID, id uuid.UUID) (*domain.Expense, error) {
	expense, err := s.GetExpense(ctx, id)
	if err != nil || expense == nil || expense.UserID != userID {
		return nil, ErrExpenseNotFound
	}
	return expense, nil
}

func (s *LedgerService) UpdateExpense(ctx context.Context, userID, id uuid.UUID, input CreateExpenseInput) (*domain.Expense, error) {
	current, err := s.GetUserExpense(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if input.Amount <= 0 {
		return nil, ErrInvalidAmount
	}
	if input.Date.IsZero() {
		input.Date = current.Date
	}

	current.Amount = input.Amount
	current.Category = input.Category
	current.Merchant = input.Merchant
	current.PaymentMethod = input.PaymentMethod
	current.Note = input.Note
	current.Location = input.Location
	current.Latitude = input.Latitude
	current.Longitude = input.Longitude
	current.Date = input.Date
	current.OutflowType = input.OutflowType
	current.HouseholdID = input.HouseholdID
	current.IsShared = input.OutflowType == domain.OutflowSharedHousehold
	current.PersonalShare = input.Amount
	current.RecoverableAmount = 0
	if input.OutflowType == domain.OutflowRecoverable {
		current.PersonalShare = 0
		current.RecoverableAmount = input.Amount
	}
	if err := s.expenseRepo.Update(ctx, current); err != nil {
		return nil, err
	}
	return current, nil
}

func (s *LedgerService) ListExpenses(ctx context.Context, filter domain.ExpenseFilter) ([]*domain.Expense, int, error) {
	return s.expenseRepo.List(ctx, filter)
}

func (s *LedgerService) DeleteExpense(ctx context.Context, userID, id uuid.UUID) error {
	if _, err := s.GetUserExpense(ctx, userID, id); err != nil {
		return err
	}
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

func (s *LedgerService) notifyHouseholdSpend(householdID uuid.UUID, payerID uuid.UUID, expense *domain.Expense) {
	if s.userRepo == nil || s.householdRepo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	household, err := s.householdRepo.GetByID(ctx, householdID)
	householdName := "Your flat"
	if err == nil && household != nil && household.Name != "" {
		householdName = household.Name
	}

	payer, err := s.userRepo.GetByID(ctx, payerID)
	payerName := "A flatmate"
	if err == nil && payer != nil && payer.FullName != "" {
		payerName = payer.FullName
	}

	itemDesc := expense.Merchant
	if itemDesc == "" && expense.Note != nil && *expense.Note != "" {
		itemDesc = *expense.Note
	}
	if itemDesc == "" {
		itemDesc = "Shared Expense"
	}

	type expoPushMessage struct {
		To        string                 `json:"to"`
		Title     string                 `json:"title"`
		Body      string                 `json:"body"`
		Sound     string                 `json:"sound"`
		Priority  string                 `json:"priority"`
		ChannelID string                 `json:"channelId"`
		Data      map[string]interface{} `json:"data,omitempty"`
	}

	var messages []expoPushMessage

	for _, split := range expense.Splits {
		if split.UserID == payerID {
			continue
		}
		member, err := s.userRepo.GetByID(ctx, split.UserID)
		if err != nil || member == nil || member.PushToken == nil || *member.PushToken == "" {
			continue
		}
		token := strings.TrimSpace(*member.PushToken)
		if token == "" {
			continue
		}

		messages = append(messages, expoPushMessage{
			To:        token,
			Title:     fmt.Sprintf("🤝 New Shared Spend: %s", itemDesc),
			Body:      fmt.Sprintf("%s added ₹%.0f in %s. Your share is ₹%.0f.", payerName, expense.Amount, householdName, split.Amount),
			Sound:     "default",
			Priority:  "high",
			ChannelID: "default",
			Data: map[string]interface{}{
				"type":         "household_expense",
				"expense_id":   expense.ID.String(),
				"household_id": householdID.String(),
			},
		})
	}

	if len(messages) == 0 {
		return
	}

	payload, err := json.Marshal(messages)
	if err != nil {
		return
	}

	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://exp.host/--/api/v2/push/send", bytes.NewBuffer(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}
