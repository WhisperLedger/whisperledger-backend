package v1

import (
	"net/http"
	"strconv"

	"whisperledger-backend/internal/middleware"
	"whisperledger-backend/internal/pkg/response"
	"whisperledger-backend/internal/service"
)

type DetectiveHandler struct {
	detectiveService *service.DetectiveService
	authService      *service.AuthService
}

func NewDetectiveHandler(detectiveService *service.DetectiveService, authService *service.AuthService) *DetectiveHandler {
	return &DetectiveHandler{
		detectiveService: detectiveService,
		authService:      authService,
	}
}

func (h *DetectiveHandler) DetectLeaks(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	leaks, err := h.detectiveService.DetectMoneyLeaks(r.Context(), userID)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.Success(w, http.StatusOK, "leaks analyzed", leaks)
}

func (h *DetectiveHandler) SafeToSpend(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	budget, _ := strconv.ParseFloat(r.URL.Query().Get("budget"), 64)

	safeSpend, err := h.detectiveService.CalculateSafeToSpend(r.Context(), userID, budget)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.Success(w, http.StatusOK, "safe-to-spend calculated", safeSpend)
}

func (h *DetectiveHandler) HouseholdAudit(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	user, err := h.authService.GetUser(r.Context(), userID)
	if err != nil || user.HouseholdID == nil {
		response.BadRequest(w, "user is not associated with any household", nil)
		return
	}

	month, _ := strconv.Atoi(r.URL.Query().Get("month"))
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))

	audit, err := h.detectiveService.GenerateHouseholdMonthlyAudit(r.Context(), *user.HouseholdID, month, year)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.Success(w, http.StatusOK, "household audit generated", audit)
}
