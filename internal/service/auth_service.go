package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"whisperledger-backend/internal/domain"
	"whisperledger-backend/internal/pkg/hash"
	"whisperledger-backend/internal/pkg/token"
)

var (
	ErrUserExists        = errors.New("a user with this email already exists")
	ErrInvalidCreds     = errors.New("invalid email or password")
	ErrUserNotFound     = errors.New("user not found")
	ErrAccountInactive  = errors.New("user account is inactive")
)

type RegisterInput struct {
	Email       string  `json:"email"`
	Password    string  `json:"password"`
	FullName    string  `json:"full_name"`
	PhoneNumber *string `json:"phone_number,omitempty"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResult struct {
	User   *domain.User     `json:"user"`
	Tokens *token.TokenPair `json:"tokens"`
}

type AuthService struct {
	userRepo   domain.UserRepository
	tokenMaker *token.TokenMaker
}

func NewAuthService(userRepo domain.UserRepository, tokenMaker *token.TokenMaker) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		tokenMaker: tokenMaker,
	}
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*AuthResult, error) {
	input.Email = strings.TrimSpace(strings.ToLower(input.Email))
	existing, err := s.userRepo.GetByEmail(ctx, input.Email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.PasswordHash == "$2a$10$realfirebaseuserhashplaceholdersalt" {
			hashedPassword, err := hash.HashPassword(input.Password)
			if err != nil {
				return nil, err
			}
			existing.FullName = input.FullName
			if input.PhoneNumber != nil {
				existing.PhoneNumber = input.PhoneNumber
			}
			existing.PasswordHash = hashedPassword
			_ = s.userRepo.UpdatePassword(ctx, existing.ID, hashedPassword)
			_ = s.userRepo.Update(ctx, existing)
			tokens, err := s.tokenMaker.GenerateTokenPair(existing)
			if err != nil {
				return nil, err
			}
			return &AuthResult{
				User:   existing,
				Tokens: tokens,
			}, nil
		}
		return nil, ErrUserExists
	}

	hashedPassword, err := hash.HashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		ID:           uuid.New(),
		Email:        input.Email,
		PasswordHash: hashedPassword,
		FullName:     input.FullName,
		PhoneNumber:  input.PhoneNumber,
		Role:         domain.RoleUser,
		IsActive:     true,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	tokens, err := s.tokenMaker.GenerateTokenPair(user)
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		User:   user,
		Tokens: tokens,
	}, nil
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (*AuthResult, error) {
	email := strings.TrimSpace(strings.ToLower(input.Email))
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidCreds
	}

	if !user.IsActive {
		return nil, ErrAccountInactive
	}

	if user.PasswordHash == "$2a$10$realfirebaseuserhashplaceholdersalt" {
		hashedPassword, err := hash.HashPassword(input.Password)
		if err == nil {
			_ = s.userRepo.UpdatePassword(ctx, user.ID, hashedPassword)
			user.PasswordHash = hashedPassword
		}
	} else if !hash.CheckPassword(input.Password, user.PasswordHash) {
		return nil, ErrInvalidCreds
	}

	tokens, err := s.tokenMaker.GenerateTokenPair(user)
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		User:   user,
		Tokens: tokens,
	}, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*token.TokenPair, error) {
	claims, err := s.tokenMaker.ValidateToken(refreshToken)
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil || !user.IsActive {
		return nil, ErrUserNotFound
	}

	return s.tokenMaker.GenerateTokenPair(user)
}

func (s *AuthService) GetUser(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (s *AuthService) UpdatePushToken(ctx context.Context, userID uuid.UUID, pushToken string) error {
	return s.userRepo.UpdatePushToken(ctx, userID, pushToken)
}

type SyncSessionInput struct {
	Email     string  `json:"email"`
	FullName  string  `json:"full_name"`
	AvatarURL *string `json:"avatar_url"`
}

func (s *AuthService) SyncSession(ctx context.Context, input SyncSessionInput) (*AuthResult, error) {
	email := strings.TrimSpace(strings.ToLower(input.Email))
	if email == "" {
		return nil, errors.New("email is required")
	}

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	if user == nil {
		placeholderHash, err := hash.HashPassword(uuid.New().String() + "-whisper-sync-salt")
		if err != nil {
			return nil, err
		}
		fullName := strings.TrimSpace(input.FullName)
		if fullName == "" {
			fullName = strings.Split(email, "@")[0]
		}
		user = &domain.User{
			ID:           uuid.New(),
			Email:        email,
			PasswordHash: placeholderHash,
			FullName:     fullName,
			AvatarURL:    input.AvatarURL,
			Role:         domain.RoleUser,
			IsActive:     true,
			Plan:         "starter_free",
			Status:       "active",
		}
		if err := s.userRepo.Create(ctx, user); err != nil {
			return nil, err
		}
	} else {
		if !user.IsActive {
			return nil, ErrAccountInactive
		}
		// Update profile details if newly provided
		dirty := false
		if strings.TrimSpace(input.FullName) != "" && user.FullName != strings.TrimSpace(input.FullName) {
			user.FullName = strings.TrimSpace(input.FullName)
			dirty = true
		}
		if input.AvatarURL != nil && (user.AvatarURL == nil || *user.AvatarURL != *input.AvatarURL) {
			user.AvatarURL = input.AvatarURL
			dirty = true
		}
		if dirty {
			_ = s.userRepo.Update(ctx, user)
		}
	}

	tokens, err := s.tokenMaker.GenerateTokenPair(user)
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		User:   user,
		Tokens: tokens,
	}, nil
}
