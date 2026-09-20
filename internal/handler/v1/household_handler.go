package v1

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"whisperledger-backend/internal/middleware"
	"whisperledger-backend/internal/pkg/response"
	"whisperledger-backend/internal/service"
)

type HouseholdHandler struct {
	householdService *service.HouseholdService
	authService      *service.AuthService
}

func NewHouseholdHandler(householdService *service.HouseholdService, authService *service.AuthService) *HouseholdHandler {
	return &HouseholdHandler{
		householdService: householdService,
		authService:      authService,
	}
}

func (h *HouseholdHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var body struct {
		Name     string `json:"name"`
		Currency string `json:"currency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		response.BadRequest(w, "name is required", nil)
		return
	}

	household, err := h.householdService.CreateHousehold(r.Context(), userID, body.Name, body.Currency)
	if err != nil {
		if err == service.ErrAlreadyInHousehold {
			response.Error(w, http.StatusConflict, "ALREADY_IN_HOUSEHOLD", err.Error(), nil)
			return
		}
		response.InternalError(w, err.Error())
		return
	}

	response.Success(w, http.StatusCreated, "household created successfully", household)
}

func (h *HouseholdHandler) Join(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var body struct {
		InviteCode string `json:"invite_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.InviteCode == "" {
		response.BadRequest(w, "invite_code is required", nil)
		return
	}

	household, err := h.householdService.JoinByInviteCode(r.Context(), userID, body.InviteCode)
	if err != nil {
		if err == service.ErrHouseholdNotFound {
			response.NotFound(w, "invalid invite code or household not found")
			return
		}
		if err == service.ErrAlreadyInHousehold {
			response.Error(w, http.StatusConflict, "ALREADY_IN_HOUSEHOLD", err.Error(), nil)
			return
		}
		response.InternalError(w, err.Error())
		return
	}

	response.Success(w, http.StatusOK, "joined household successfully", household)
}

func (h *HouseholdHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	user, err := h.authService.GetUser(r.Context(), userID)
	if err != nil || user.HouseholdID == nil {
		response.NotFound(w, "user is not a member of any household")
		return
	}

	household, err := h.householdService.GetHousehold(r.Context(), *user.HouseholdID)
	if err != nil {
		response.NotFound(w, "household not found")
		return
	}

	response.Success(w, http.StatusOK, "household retrieved", household)
}

func (h *HouseholdHandler) GetBalances(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "invalid household id", nil)
		return
	}

	balances, err := h.householdService.GetBalances(r.Context(), id)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.Success(w, http.StatusOK, "balances retrieved", balances)
}

func (h *HouseholdHandler) GetOptimalSettlements(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "invalid household id", nil)
		return
	}

	settlements, err := h.householdService.CalculateOptimalSettlements(r.Context(), id)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.Success(w, http.StatusOK, "optimal settlements calculated", settlements)
}

func (h *HouseholdHandler) Settle(w http.ResponseWriter, r *http.Request) {
	householdIDStr := chi.URLParam(r, "id")
	householdID, err := uuid.Parse(householdIDStr)
	if err != nil {
		response.BadRequest(w, "invalid household id", nil)
		return
	}

	var body struct {
		FromUserID uuid.UUID `json:"from_user_id"`
		ToUserID   uuid.UUID `json:"to_user_id"`
		Amount     float64   `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Amount <= 0 {
		response.BadRequest(w, "valid from_user_id, to_user_id, and amount are required", nil)
		return
	}

	if err := h.householdService.SettleDebt(r.Context(), householdID, body.FromUserID, body.ToUserID, body.Amount); err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.Success(w, http.StatusOK, "settlement recorded successfully", nil)
}
