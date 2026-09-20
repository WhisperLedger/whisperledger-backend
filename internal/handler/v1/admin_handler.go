package v1

import (
	"net/http"
	"strconv"

	"whisperledger-backend/internal/pkg/response"
	"whisperledger-backend/internal/service"
)

type AdminHandler struct {
	adminService *service.AdminService
}

func NewAdminHandler(adminService *service.AdminService) *AdminHandler {
	return &AdminHandler{adminService: adminService}
}

func (h *AdminHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := h.adminService.GetPlatformMetrics(r.Context())
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.Success(w, http.StatusOK, "platform metrics retrieved", metrics)
}

func (h *AdminHandler) Users(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	users, total, err := h.adminService.ListUsers(r.Context(), limit, offset)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.Paginated(w, users, total, limit, offset)
}
