package token

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"whisperledger-backend/internal/domain"
)

func TestTokenGenerationAndValidation(t *testing.T) {
	maker := NewTokenMaker("test-secret-key-whisperledger-123", time.Hour, 24*time.Hour)
	user := &domain.User{
		ID:       uuid.New(),
		Email:    "test@whisperledger.com",
		FullName: "Alex Vance",
		Role:     domain.RoleUser,
		IsActive: true,
	}

	pair, err := maker.GenerateTokenPair(user)
	if err != nil {
		t.Fatalf("Failed to generate token pair: %v", err)
	}

	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatalf("Token pair contains empty token")
	}

	claims, err := maker.ValidateToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("Failed to validate access token: %v", err)
	}

	if claims.UserID != user.ID || claims.Email != user.Email {
		t.Errorf("Claims mismatch. Got %+v, expected user ID %s", claims, user.ID)
	}
}
