package v1

import (
	"encoding/json"
	"net/http"

	"whisperledger-backend/internal/middleware"
	"whisperledger-backend/internal/pkg/response"
	"whisperledger-backend/internal/service"
)

type ProductHandler struct{ productService *service.ProductService }

func NewProductHandler(productService *service.ProductService) *ProductHandler {
	return &ProductHandler{productService: productService}
}

func (h *ProductHandler) CreateSupportCase(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Subject string `json:"subject"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
		response.BadRequest(w, "invalid request body", nil)
		return
	}
	item, err := h.productService.CreateSupportCase(r.Context(), middleware.GetUserID(r.Context()), body.Subject, body.Message)
	if err != nil {
		if err == service.ErrInvalidSupportCase {
			response.BadRequest(w, err.Error(), nil)
			return
		}
		response.InternalError(w, "could not create support request")
		return
	}
	response.Success(w, http.StatusCreated, "support request received", item)
}
