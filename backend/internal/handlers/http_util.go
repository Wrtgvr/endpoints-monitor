package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/wrtgvr/websites-monitor/internal/domain"
)

// shortcut for `http.Error()`
func (h *HTTPHandler) error(w http.ResponseWriter, code int, msg string) {
	http.Error(w, msg, code)
}

// shortcut for `HttpHandler.error()` with code 500 and msg `internal server error`
func (h *HTTPHandler) internalError(w http.ResponseWriter) {
	h.error(w, http.StatusInternalServerError, "internal server error")
}

// Decode request body to `v`.
// Response with BadRequest on decode error
func (h *HTTPHandler) decodeJSONRequestBody(w http.ResponseWriter, r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return err
	}
	return nil
}

// Encode `v` to `w`
// Response with InternalServerError on encode error
// Response with `successCode` on successful encoding
func (h *HTTPHandler) encodeJSONResponse(w http.ResponseWriter, v any, successCode int) error {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return err
	}
	w.WriteHeader(successCode)
	return nil
}

func (h *HTTPHandler) domainEndpointToDTO(ep *domain.Endpoint) *EndpointResponse {
	return &EndpointResponse{
		ID:           ep.ID,
		Name:         ep.Name,
		URL:          ep.URL,
		Status:       ep.Status,
		LastChecked:  ep.LastChecked,
		ResponseTime: ep.ResponseTime,
	}
}

func (h *HTTPHandler) domainEndpointStatusToDTO(ep *domain.EndpointStatus) *EndpointStatusResponse {
	return &EndpointStatusResponse{
		ID:           ep.ID,
		Status:       ep.Status,
		LastChecked:  ep.LastChecked,
		ResponseTime: ep.ResponseTime,
	}
}

func (h *HTTPHandler) domainEndpointInfoToDTO(ep *domain.EndpointInfo) *EndpointInfoResponse {
	return &EndpointInfoResponse{
		ID:   ep.ID,
		Name: ep.Name,
		URL:  ep.URL,
	}
}
