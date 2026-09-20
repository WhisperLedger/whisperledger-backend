package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"whisperledger-backend/internal/domain"
)

var (
	ErrReceivableNotFound = errors.New("receivable record not found")
)

type CreateReceivableInput struct {
	Title           string                `json:"title"`
	DebtorName      string                `json:"debtor_name"`
	DebtorPhone     *string               `json:"debtor_phone,omitempty"`
	Amount          float64               `json:"amount"`
	Type            domain.ReceivableType `json:"type"`
	DueDate         *time.Time            `json:"due_date,omitempty"`
	SourceExpenseID *uuid.UUID            `json:"source_expense_id,omitempty"`
	Notes           *string               `json:"notes,omitempty"`
}

type SettleReceivableInput struct {
	Amount float64 `json:"amount"`
}

type RecoveryService struct {
	repo domain.ReceivableRepository
}

func NewRecoveryService(repo domain.ReceivableRepository) *RecoveryService {
	return &RecoveryService{repo: repo}
}

func (s *RecoveryService) Create(ctx context.Context, userID uuid.UUID, input CreateReceivableInput) (*domain.Receivable, error) {
	if input.Amount <= 0 {
		return nil, ErrInvalidAmount
	}

	rec := &domain.Receivable{
		ID:              uuid.New(),
		UserID:          userID,
		Title:           input.Title,
		DebtorName:      input.DebtorName,
		DebtorPhone:     input.DebtorPhone,
		Amount:          input.Amount,
		SettledAmount:   0,
		Type:            input.Type,
		Status:          domain.ReceivablePending,
		DueDate:         input.DueDate,
		SourceExpenseID: input.SourceExpenseID,
		Notes:           input.Notes,
	}

	if err := s.repo.Create(ctx, rec); err != nil {
		return nil, err
	}

	return rec, nil
}

func (s *RecoveryService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Receivable, error) {
	rec, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, ErrReceivableNotFound
	}
	return rec, nil
}

func (s *RecoveryService) List(ctx context.Context, filter domain.ReceivableFilter) ([]*domain.Receivable, int, error) {
	return s.repo.List(ctx, filter)
}

func (s *RecoveryService) Settle(ctx context.Context, id uuid.UUID, amount float64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	return s.repo.Settle(ctx, id, amount)
}

func (s *RecoveryService) GetTotalPending(ctx context.Context, userID uuid.UUID) (float64, error) {
	return s.repo.GetTotalPending(ctx, userID)
}
