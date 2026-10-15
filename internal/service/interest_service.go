package service

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"whisperledger-backend/internal/domain"
)

var ErrInvalidInterestEmail = errors.New("enter a valid email address")

type InterestService struct {
	repository domain.InterestRepository
}

func NewInterestService(repository domain.InterestRepository) *InterestService {
	return &InterestService{repository: repository}
}

func (s *InterestService) Subscribe(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) == 0 || len(email) > 254 {
		return ErrInvalidInterestEmail
	}

	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || !strings.Contains(parsed.Address, "@") {
		return ErrInvalidInterestEmail
	}

	return s.repository.Subscribe(ctx, email)
}
