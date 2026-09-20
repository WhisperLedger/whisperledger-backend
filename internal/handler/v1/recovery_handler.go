package v1

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"whisperledger-backend/internal/domain"
	"whisperledger-backend/internal/middleware"
	"whisperledger-backend/internal/pkg/response"
	"whisperledger-backend/internal/service"
)

type RecoveryHandler struct {
	recoveryService *service.RecoveryService
}

func NewRecoveryHandler(recoveryService *service.RecoveryService) *RecoveryHandler {
	return &RecoveryHandler{recoveryService: recoveryService}
}

func (h *RecoveryHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var input service.CreateReceivableInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, "invalid request body", err.Error())
		return
	}

	if input.Title == "" || input.DebtorName == "" || input.Amount <= 0 {
		response.BadRequest(w, "title, debtor_name, and valid amount are required", nil)
		return
	}

	receivable, err := h.recoveryService.Create(r.Context(), userID, input)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.Success(w, http.StatusCreated, "receivable created successfully", receivable)
}

func (h *RecoveryHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	filter := domain.ReceivableFilter{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	}

	if st := r.URL.Query().Get("status"); st != "" {
		status := domain.ReceivableStatus(st)
		filter.Status = &status
	}

	if tp := r.URL.Query().Get("type"); tp != "" {
		t := domain.ReceivableType(tp)
		filter.Type = &t
	}

	list, total, err := h.recoveryService.List(r.Context(), filter)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.Paginated(w, list, total, limit, offset)
}

func (h *RecoveryHandler) Settle(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "invalid receivable id", nil)
		return
	}

	var body service.SettleReceivableInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Amount <= 0 {
		response.BadRequest(w, "valid positive amount required", nil)
		return
	}

	if err := h.recoveryService.Settle(r.Context(), id, body.Amount); err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.Success(w, http.StatusOK, "receivable settled successfully", nil)
}

func (h *RecoveryHandler) Summary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	totalPending, err := h.recoveryService.GetTotalPending(r.Context(), userID)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.Success(w, http.StatusOK, "summary retrieved", map[string]interface{}{
		"total_pending_recoverable": totalPending,
	})
}
