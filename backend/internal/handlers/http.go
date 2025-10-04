package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wrtgvr/websites-monitor/internal/domain"
	errs "github.com/wrtgvr/websites-monitor/internal/errors"
	"github.com/wrtgvr/websites-monitor/internal/monitor"
	"github.com/wrtgvr/websites-monitor/internal/storage"
)

type HTTPHandler struct {
	storage         storage.EndpointsStorage
	responseTimeout time.Duration
}

func NewHTTPHandler(storage storage.EndpointsStorage, responseTimeount time.Duration) *HTTPHandler {
	return &HTTPHandler{
		storage:         storage,
		responseTimeout: responseTimeount,
	}
}

// GET /api/endpoints
func (h *HTTPHandler) GetEndpoints(w http.ResponseWriter, r *http.Request) {
	//* query params
	var limit, offset int64
	var defLimit int64 = 20
	var defOffset int64 = 0
	// limit
	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		limit = defLimit
	}
	if v, err := strconv.ParseInt(limitStr, 10, 64); err != nil {
		limit = defLimit
	} else {
		limit = v
	}
	// offset
	offsetStr := r.URL.Query().Get("offset")
	if offsetStr == "" {
		offset = defOffset
	}
	if v, err := strconv.ParseInt(offsetStr, 10, 64); err != nil {
		offset = defOffset
	} else {
		offset = v
	}

	//* storage request
	ctx, cancel := context.WithTimeout(context.Background(), h.responseTimeout)
	defer cancel()

	domainEps, err := h.storage.GetEndpoints(ctx, limit, offset)
	if err != nil {
		h.internalError(w)
		return
	}

	//* http response
	endpoints := make([]*EndpointResponse, len(domainEps))
	for i, ep := range domainEps {
		endpoints[i] = h.domainEndpointToDTO(ep)
	}

	h.encodeJSONResponse(w, endpoints, http.StatusOK)
}

// POST /api/endpoints
func (h *HTTPHandler) PostEndpoint(w http.ResponseWriter, r *http.Request) {
	//* decode request
	var req CreateEndpointRequest
	if err := h.decodeJSONRequestBody(w, r, &req); err != nil {
		return
	}

	//* storage request
	ctx, cancel := context.WithTimeout(context.Background(), h.responseTimeout)
	defer cancel()

	if err := h.storage.AddEndpoint(ctx, &domain.EndpointInfo{
		Name: req.Name,
		URL:  req.URL,
	}); err != nil {
		if err.Type == errs.TypeInternal {
			h.internalError(w)
			return
		}
		h.error(w, err.Code, err.Msg)
		return
	}

	//* http response
	w.WriteHeader(http.StatusCreated)
}

// PATCH /api/endpoints/{id}
func (h *HTTPHandler) PatchEndpoint(w http.ResponseWriter, r *http.Request) {
	//* get id from path
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		h.error(w, http.StatusBadRequest, "id is required")
	}

	//* decode request
	var req UpdateEndpointInfoRequest
	if err := h.decodeJSONRequestBody(w, r, &req); err != nil {
		return
	}

	//* check request
	if id == "" {
		h.error(w, http.StatusBadRequest, "id is required")
		return
	}
	if req.Name == "" && req.URL == "" {
		h.error(w, http.StatusBadRequest, "required either new name or new url")
		return
	}

	//* storage request
	ctx, cancel := context.WithTimeout(context.Background(), h.responseTimeout)
	defer cancel()

	err := h.storage.ChangeEndpointInfo(ctx, &domain.EndpointInfo{
		ID:   id,
		Name: req.Name,
		URL:  req.URL,
	})
	if err != nil {
		h.error(w, err.Code, err.Msg)
	}
}

// DELETE /api/endpoints/{id}
func (h *HTTPHandler) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	//* get id from path
	id := r.PathValue("id")
	if strings.TrimSpace(id) == "" {
		h.error(w, http.StatusBadRequest, "id is required")
	}

	//* storage request
	ctx, cancel := context.WithTimeout(context.Background(), h.responseTimeout)
	defer cancel()

	err := h.storage.DeleteEndpoint(ctx, id)
	if err != nil {
		h.error(w, err.Code, err.Msg)
		return
	}

	//* response
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/monitor-sse
func (h *HTTPHandler) MonitorSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	mntr := monitor.NewMonitor(h.storage)
	go mntr.Run()

	rc := http.NewResponseController(w)

	for {
		select {
		case <-r.Context().Done():
			mntr.Stop()
			return
		case epStatus := <-mntr.Out:
			// endpoint status recieved
			b, err := json.Marshal(epStatus)
			if err != nil {
				mntr.Stop()
				return
			}
			_, err = fmt.Fprintf(w, "event: %s\n\ndata: %s\n\n", "pingresult", b)
			if err != nil {
				mntr.Stop()
				return
			}

			err = rc.Flush()
			if err != nil {
				mntr.Stop()
				return
			}
		default:
			// Heartbeat
			fmt.Fprintf(w, "event: %s\n\ndata: %s\n\n", "heartbeat", "Heartbeat")
			rc.Flush()

			time.Sleep(5 * time.Second)
		}
	}
}
