package v1

import (
	"encoding/json"
	"net/http"
	"strings"

	"whisperledger-backend/internal/pkg/response"
	"whisperledger-backend/internal/service"
)

type InterestHandler struct {
	interestService *service.InterestService
}

func NewInterestHandler(interestService *service.InterestService) *InterestHandler {
	return &InterestHandler{interestService: interestService}
}

type interestRequest struct {
	Email   string `json:"email"`
	Consent bool   `json:"consent"`
	Company string `json:"company"` // Honeypot: legitimate users never fill this field.
}

func (h *InterestHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	defer r.Body.Close()

	var request interestRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		response.BadRequest(w, "invalid request", nil)
		return
	}

	// A filled honeypot is treated as accepted. This quietly stops basic form
	// bots without giving them feedback they can use to tune their requests.
	if strings.TrimSpace(request.Company) != "" {
		response.Success(w, http.StatusAccepted, "request received", nil)
		return
	}
	if !request.Consent {
		response.BadRequest(w, "consent is required to receive updates", nil)
		return
	}
	if err := h.interestService.Subscribe(r.Context(), request.Email); err != nil {
		if err == service.ErrInvalidInterestEmail {
			response.BadRequest(w, err.Error(), nil)
			return
		}
		response.InternalError(w, "unable to save your request")
		return
	}

	response.Success(w, http.StatusAccepted, "request received", nil)
}
