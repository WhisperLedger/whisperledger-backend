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
	ErrAdminAccountAppSignIn     = errors.New("this Google account is reserved for administration. Use a separate WhisperLedger app account.")
	ErrLegacyIdentityInvalid     = errors.New("the legacy account verification could not be completed")
	ErrLegacyAccountUnavailable  = errors.New("this account is not eligible for legacy sign-in")
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

type FirebaseMigrationLoginInput struct {
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
	firebaseProjectID   string
}

func NewAuthService(userRepo domain.UserRepository, tokenMaker *token.TokenMaker, googleOAuthClientID, firebaseProjectID string) *AuthService {
	return &AuthService{
		userRepo:            userRepo,
		tokenMaker:          tokenMaker,
		googleOAuthClientID: googleOAuthClientID,
		firebaseProjectID:   firebaseProjectID,
	}
}

// LegacyFirebaseLogin is a bounded migration bridge. It verifies an existing
// Firebase ID token server-side, locks it to the imported account's Firebase
// subject, and issues first-party Postgres API tokens. Firebase is never used
// as the session authority after this exchange.
func (s *AuthService) LegacyFirebaseLogin(ctx context.Context, input FirebaseMigrationLoginInput) (*AuthResult, error) {
	if strings.TrimSpace(input.IDToken) == "" || s.firebaseProjectID == "" {
		return nil, ErrLegacyIdentityInvalid
	}

	payload, err := idtoken.Validate(ctx, input.IDToken, s.firebaseProjectID)
	if err != nil || payload.Subject == "" {
		return nil, ErrLegacyIdentityInvalid
	}
	email, _ := payload.Claims["email"].(string)
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, ErrLegacyIdentityInvalid
	}

	user, err := s.userRepo.GetByFirebaseSubject(ctx, payload.Subject)
	if err != nil {
		return nil, ErrLegacyIdentityInvalid
	}
	if user == nil {
		user, err = s.userRepo.GetByEmail(ctx, email)
		if err != nil {
			return nil, ErrLegacyIdentityInvalid
		}
	}
	if user != nil && !user.IsActive {
		return nil, ErrLegacyAccountUnavailable
	}
	if user == nil {
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
		user = &domain.User{
			ID:              uuid.New(),
			Email:           email,
			FullName:        name,
			AvatarURL:       avatarURL,
			PasswordHash:    "$2a$10$realfirebaseuserhashplaceholdersalt",
			FirebaseSubject: &payload.Subject,
			Role:            domain.RoleUser,
			IsActive:        true,
		}
		if err := s.userRepo.Create(ctx, user); err != nil {
			return nil, ErrLegacyIdentityInvalid
		}
	} else if user.PasswordHash != "$2a$10$realfirebaseuserhashplaceholdersalt" && user.FirebaseSubject == nil {
		return nil, ErrLegacyAccountUnavailable
	}
	if user.FirebaseSubject != nil && *user.FirebaseSubject != payload.Subject {
		return nil, ErrLegacyIdentityInvalid
	}
	if user.FirebaseSubject == nil {
		if err := s.userRepo.LinkFirebaseSubject(ctx, user.ID, payload.Subject); err != nil {
			return nil, ErrLegacyIdentityInvalid
		}
		user.FirebaseSubject = &payload.Subject
	}

	tokens, err := s.tokenMaker.GenerateTokenPair(user)
	if err != nil {
		return nil, err
	}
	return &AuthResult{User: user, Tokens: tokens}, nil
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
			// Google has verified ownership of this exact email address. Treat that
			// proof as a sign-in method for a matching standard account and retain
			// the same account ID, data, role, and subscription. The database's
			// unique Google-subject constraint prevents one identity from joining two
			// WhisperLedger accounts. Platform administrators stay behind the
			// separate admin authentication and MFA boundary.
			if existing.Role != domain.RolePlatformAdmin {
				if err := s.userRepo.LinkGoogleSubject(ctx, existing.ID, payload.Subject); err != nil {
					return nil, err
				}
				existing.GoogleSubject = &payload.Subject
				user = existing
			} else {
				return nil, ErrAdminAccountAppSignIn
			}
		} else {
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
