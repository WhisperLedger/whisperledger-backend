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

func (h *AdminHandler) Telemetry(w http.ResponseWriter, r *http.Request) {
	telemetry, err := h.adminService.GetCloudTelemetry(r.Context())
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.Success(w, http.StatusOK, "cloud telemetry and metrics retrieved", telemetry)
}

func (h *AdminHandler) Users(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	users, total, err := h.adminService.ListUsers(r.Context(), limit, offset)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	if users == nil {
		users = []*domain.User{}
	}

	response.Paginated(w, users, total, limit, offset)
}

type updateSubscriptionReq struct {
	Plan   string `json:"plan"`
	Status string `json:"status"`
}

func (h *AdminHandler) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	userID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "invalid user id", nil)
		return
	}

	var req updateSubscriptionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body", nil)
		return
	}

	if req.Plan == "" {
		req.Plan = "starter_free"
	}
	if req.Status == "" {
		req.Status = "active"
	}

	if err := h.adminService.UpdateUserSubscription(r.Context(), userID, req.Plan, req.Status); err != nil {
		response.InternalError(w, err.Error())
		return
	}
	h.adminService.RecordAudit(r.Context(), middleware.GetUserID(r.Context()), "subscription.updated", "user", userID.String(), map[string]interface{}{
		"plan": req.Plan, "status": req.Status,
	})

	response.Success(w, http.StatusOK, "user subscription updated successfully", map[string]interface{}{
		"user_id": userID,
		"plan":    req.Plan,
		"status":  req.Status,
	})
}

type createNotificationReq struct {
	AppName        string      `json:"app_name"`
	Title          string      `json:"title"`
	Body           string      `json:"body"`
	ImageURL       string      `json:"image_url"`
	Target         string      `json:"target"`
	TargetUserID   *uuid.UUID  `json:"target_user_id"`
	TargetUserIDs  []uuid.UUID `json:"target_user_ids"`
	Type           string      `json:"type"`
	DeliveredCount int         `json:"delivered_count"`
}

func (h *AdminHandler) CreateNotification(w http.ResponseWriter, r *http.Request) {
	var req createNotificationReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body", nil)
		return
	}

	if req.Title == "" || req.Body == "" {
		response.BadRequest(w, "title and body are required", nil)
		return
	}
	if req.Target == "" {
		req.Target = "all"
	}
	if req.Target != "all" && req.Target != "pro_only" && req.Target != "single_user" && req.Target != "selected_users" {
		response.BadRequest(w, "invalid notification target", nil)
		return
	}
	if req.Target == "single_user" && req.TargetUserID == nil {
		response.BadRequest(w, "a user is required for a single-user notification", nil)
		return
	}
	if req.Target == "selected_users" && (len(req.TargetUserIDs) == 0 || len(req.TargetUserIDs) > 100) {
		response.BadRequest(w, "select between 1 and 100 users", nil)
		return
	}
	if req.Type == "" {
		req.Type = "system"
	}

	notif := &domain.BroadcastNotification{
		AppName:       req.AppName,
		Title:         req.Title,
		Body:          req.Body,
		ImageURL:      req.ImageURL,
		Target:        req.Target,
		TargetUserID:  req.TargetUserID,
		TargetUserIDs: req.TargetUserIDs,
		Type:          req.Type,
		// Recipient counts are calculated from valid server-side push tokens.
		// Do not trust a count supplied by the console client.
		DeliveredCount: 0,
		DispatchStatus: "delivered",
	}

	if err := h.adminService.CreateBroadcastNotification(r.Context(), notif); err != nil {
		response.InternalError(w, err.Error())
		return
	}
	h.adminService.RecordAudit(r.Context(), middleware.GetUserID(r.Context()), "notification.broadcast", "broadcast_notification", notif.ID.String(), map[string]interface{}{
		"target": notif.Target, "type": notif.Type, "delivered_count": notif.DeliveredCount,
	})

	response.Success(w, http.StatusCreated, "push notification broadcast recorded successfully", notif)
}

func (h *AdminHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	notifications, err := h.adminService.ListBroadcastNotifications(r.Context(), limit)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}

	response.Success(w, http.StatusOK, "broadcast notifications retrieved", notifications)
}
