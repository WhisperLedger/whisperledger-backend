package v1

import (
	"encoding/json"
	"net/http"

	"whisperledger-backend/internal/middleware"
	"whisperledger-backend/internal/pkg/response"
	"whisperledger-backend/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var input service.RegisterInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, "invalid request body", err.Error())
		return
	}

	if input.Email == "" || input.Password == "" || input.FullName == "" {
		response.BadRequest(w, "email, password, and full_name are required", nil)
		return
	}

	result, err := h.authService.Register(r.Context(), input)
	if err != nil {
		if err == service.ErrUserExists {
			response.Error(w, http.StatusConflict, "USER_EXISTS", err.Error(), nil)
			return
		}
		response.InternalError(w, err.Error())
		return
	}

	response.Success(w, http.StatusCreated, "user registered successfully", result)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var input service.LoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, "invalid request body", err.Error())
		return
	}

	if input.Email == "" || input.Password == "" {
		response.BadRequest(w, "email and password are required", nil)
		return
	}

	result, err := h.authService.Login(r.Context(), input)
	if err != nil {
		if err == service.ErrInvalidCreds || err == service.ErrAccountInactive {
			response.Unauthorized(w, err.Error())
			return
		}
		response.InternalError(w, err.Error())
		return
	}

	response.Success(w, http.StatusOK, "login successful", result)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RefreshToken == "" {
		response.BadRequest(w, "refresh_token is required", nil)
		return
	}

	tokens, err := h.authService.Refresh(r.Context(), body.RefreshToken)
	if err != nil {
		response.Unauthorized(w, "invalid or expired refresh token")
		return
	}

	response.Success(w, http.StatusOK, "token refreshed", tokens)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	user, err := h.authService.GetUser(r.Context(), userID)
	if err != nil {
		response.NotFound(w, "user profile not found")
		return
	}

	response.Success(w, http.StatusOK, "user profile retrieved", user)
}
