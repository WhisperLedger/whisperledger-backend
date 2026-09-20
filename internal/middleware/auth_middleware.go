package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"whisperledger-backend/internal/domain"
	"whisperledger-backend/internal/pkg/response"
	"whisperledger-backend/internal/pkg/token"
)

type contextKey string

const (
	UserClaimsKey contextKey = "user_claims"
)

type AuthMiddleware struct {
	tokenMaker *token.TokenMaker
}

func NewAuthMiddleware(tokenMaker *token.TokenMaker) *AuthMiddleware {
	return &AuthMiddleware{tokenMaker: tokenMaker}
}

func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			response.Unauthorized(w, "missing Authorization header")
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			response.Unauthorized(w, "invalid Authorization header format, must be Bearer <token>")
			return
		}

		claims, err := m.tokenMaker.ValidateToken(parts[1])
		if err != nil {
			response.Unauthorized(w, err.Error())
			return
		}

		ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *AuthMiddleware) RequireRole(roles ...domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value(UserClaimsKey).(*token.Claims)
			if !ok || claims == nil {
				response.Unauthorized(w, "unauthorized access")
				return
			}

			allowed := false
			for _, role := range roles {
				if claims.Role == role {
					allowed = true
					break
				}
			}

			if !allowed {
				response.Forbidden(w, "insufficient privileges")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func GetClaims(ctx context.Context) *token.Claims {
	if claims, ok := ctx.Value(UserClaimsKey).(*token.Claims); ok {
		return claims
	}
	return nil
}

func GetUserID(ctx context.Context) uuid.UUID {
	if claims := GetClaims(ctx); claims != nil {
		return claims.UserID
	}
	return uuid.Nil
}
