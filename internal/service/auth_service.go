package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/api/idtoken"
	"whisperledger-backend/internal/domain"
	"whisperledger-backend/internal/pkg/hash"
	"whisperledger-backend/internal/pkg/token"
)

var (
	ErrUserExists                = errors.New("a user with this email already exists")
	ErrInvalidCreds              = errors.New("invalid email or password")
	ErrUserNotFound              = errors.New("user not found")
	ErrAccountInactive           = errors.New("user account is inactive")
	ErrMigrationReset            = errors.New("this migrated account must complete a verified password reset")
	ErrGoogleTokenInvalid        = errors.New("the Google sign-in token is invalid or expired")
	ErrGoogleAccountLinkRequired = errors.New("sign in with your password first before linking Google to this existing account")
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

type GoogleLoginInput struct {
	IDToken string `json:"id_token"`
}

type AuthResult struct {
	User   *domain.User     `json:"user"`
	Tokens *token.TokenPair `json:"tokens"`
}

type AuthService struct {
	userRepo            domain.UserRepository
	tokenMaker          *token.TokenMaker
	googleOAuthClientID string
}

func NewAuthService(userRepo domain.UserRepository, tokenMaker *token.TokenMaker, googleOAuthClientID string) *AuthService {
	return &AuthService{
		userRepo:            userRepo,
		tokenMaker:          tokenMaker,
		googleOAuthClientID: googleOAuthClientID,
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
			return nil, ErrMigrationReset
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
		return nil, ErrMigrationReset
	}
	if !hash.CheckPassword(input.Password, user.PasswordHash) {
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

// GoogleLogin accepts only an ID token whose signature, issuer, expiry and
// audience have been verified by Google's official token validator. The Google
// subject, rather than the mutable email address, is the account identifier.
func (s *AuthService) GoogleLogin(ctx context.Context, input GoogleLoginInput) (*AuthResult, error) {
	if strings.TrimSpace(input.IDToken) == "" || s.googleOAuthClientID == "" {
		return nil, ErrGoogleTokenInvalid
	}

	payload, err := idtoken.Validate(ctx, input.IDToken, s.googleOAuthClientID)
	if err != nil || payload.Subject == "" {
		return nil, ErrGoogleTokenInvalid
	}

	email, _ := payload.Claims["email"].(string)
	email = strings.TrimSpace(strings.ToLower(email))
	emailVerified, _ := payload.Claims["email_verified"].(bool)
	if email == "" || !emailVerified {
		return nil, ErrGoogleTokenInvalid
	}

	user, err := s.userRepo.GetByGoogleSubject(ctx, payload.Subject)
	if err != nil {
		return nil, err
	}
	if user == nil {
		// Never silently attach a new Google identity to a password account by
		// email alone. The existing account holder must prove control by signing
		// in normally before an explicit account-linking flow is introduced.
		existing, err := s.userRepo.GetByEmail(ctx, email)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, ErrGoogleAccountLinkRequired
		}

		name, _ := payload.Claims["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			name = strings.Split(email, "@")[0]
		}
		picture, _ := payload.Claims["picture"].(string)
		var avatarURL *string
		if strings.TrimSpace(picture) != "" {
			avatarURL = &picture
		}

		// Password authentication remains unavailable for an OAuth-only account
		// until a dedicated verified password-creation flow is used.
		generatedPassword, err := hash.HashPassword(uuid.NewString())
		if err != nil {
			return nil, err
		}
		user = &domain.User{
			ID:            uuid.New(),
			Email:         email,
			PasswordHash:  generatedPassword,
			FullName:      name,
			AvatarURL:     avatarURL,
			GoogleSubject: &payload.Subject,
			Role:          domain.RoleUser,
			IsActive:      true,
		}
		if err := s.userRepo.Create(ctx, user); err != nil {
			return nil, err
		}
	}

	if !user.IsActive {
		return nil, ErrAccountInactive
	}
	tokens, err := s.tokenMaker.GenerateTokenPair(user)
	if err != nil {
		return nil, err
	}
	return &AuthResult{User: user, Tokens: tokens}, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*token.TokenPair, error) {
	claims, err := s.tokenMaker.ValidateRefreshToken(refreshToken)
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

func (s *AuthService) UpdateProfile(ctx context.Context, userID uuid.UUID, fullName string, avatarURL *string) (*domain.User, error) {
	user, err := s.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if name := strings.TrimSpace(fullName); name != "" {
		user.FullName = name
	}
	if avatarURL != nil {
		user.AvatarURL = avatarURL
	}
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}
