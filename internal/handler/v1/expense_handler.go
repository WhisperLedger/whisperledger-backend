package v1

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"whisperledger-backend/internal/domain"
	"whisperledger-backend/internal/middleware"
	"whisperledger-backend/internal/pkg/response"
	"whisperledger-backend/internal/service"
)

type ExpenseHandler struct {
	ledgerService *service.LedgerService
}

func NewExpenseHandler(ledgerService *service.LedgerService) *ExpenseHandler {
	return &ExpenseHandler{ledgerService: ledgerService}
}

func (h *ExpenseHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var input service.CreateExpenseInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, "invalid request body", err.Error())
		return
	}

	if input.Amount <= 0 {
		response.BadRequest(w, "amount must be greater than 0", nil)
		return
	}
	if input.Merchant == "" {
		response.BadRequest(w, "merchant name is required", nil)
		return
	}

	expense, err := h.ledgerService.CreateExpense(r.Context(), userID, input)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.Success(w, http.StatusCreated, "expense recorded successfully", expense)
}

func (h *ExpenseHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	filter := domain.ExpenseFilter{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	}

	if hIDStr := r.URL.Query().Get("household_id"); hIDStr != "" {
		if hID, err := uuid.Parse(hIDStr); err == nil {
			filter.HouseholdID = &hID
		}
	}

	if ot := r.URL.Query().Get("outflow_type"); ot != "" {
		outflowType := domain.OutflowType(ot)
		filter.OutflowType = &outflowType
	}

	if cat := r.URL.Query().Get("category"); cat != "" {
		category := domain.ExpenseCategory(cat)
		filter.Category = &category
	}

	if startStr := r.URL.Query().Get("start_date"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			filter.StartDate = &t
		}
	}

	if endStr := r.URL.Query().Get("end_date"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			filter.EndDate = &t
		}
	}

	expenses, total, err := h.ledgerService.ListExpenses(r.Context(), filter)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.Paginated(w, expenses, total, limit, offset)
}

func (h *ExpenseHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "invalid expense id", nil)
		return
	}

	expense, err := h.ledgerService.GetExpense(r.Context(), id)
	if err != nil {
		response.NotFound(w, "expense not found")
		return
	}

	response.Success(w, http.StatusOK, "expense retrieved", expense)
}

func (h *ExpenseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "invalid expense id", nil)
		return
	}

	if err := h.ledgerService.DeleteExpense(r.Context(), id); err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.Success(w, http.StatusOK, "expense deleted", nil)
}

func (h *ExpenseHandler) Summary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var startDate, endDate time.Time

	if startStr := r.URL.Query().Get("start_date"); startStr != "" {
		startDate, _ = time.Parse(time.RFC3339, startStr)
	}
	if endStr := r.URL.Query().Get("end_date"); endStr != "" {
		endDate, _ = time.Parse(time.RFC3339, endStr)
	}

	summary, err := h.ledgerService.GetSummary(r.Context(), userID, startDate, endDate)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.Success(w, http.StatusOK, "summary retrieved", summary)
}
