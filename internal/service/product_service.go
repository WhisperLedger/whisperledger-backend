package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrInvalidSupportCase = errors.New("subject and message are required")

type ProductService struct{ pool *pgxpool.Pool }

func NewProductService(pool *pgxpool.Pool) *ProductService { return &ProductService{pool: pool} }

type SupportCase struct {
	ID      uuid.UUID `json:"id"`
	Subject string    `json:"subject"`
	Message string    `json:"message"`
	Status  string    `json:"status"`
}

func (s *ProductService) CreateSupportCase(ctx context.Context, userID uuid.UUID, subject, message string) (*SupportCase, error) {
	subject, message = strings.TrimSpace(subject), strings.TrimSpace(message)
	if subject == "" || message == "" {
		return nil, ErrInvalidSupportCase
	}
	if len(subject) > 255 || len(message) > 8000 {
		return nil, errors.New("support request is too long")
	}
	item := &SupportCase{ID: uuid.New(), Subject: subject, Message: message, Status: "received"}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO support_cases (id, user_id, subject, message, status)
		VALUES ($1, $2, $3, $4, $5)
	`, item.ID, userID, item.Subject, item.Message, item.Status)
	if err != nil {
		return nil, err
	}
	return item, nil
}
