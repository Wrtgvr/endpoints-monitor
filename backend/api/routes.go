package api

import (
	"net/http"

	"github.com/wrtgvr/websites-monitor/internal/handlers"
)

func RegisterRoutes(mux *http.ServeMux, h handlers.Handler) {
	mux.HandleFunc("GET /api/endpoints", h.GetEndpoints)
	mux.HandleFunc("POST /api/endpoints", h.PostEndpoint)
	mux.HandleFunc("PATCH /api/endpoints/{id}", h.PatchEndpoint)
	mux.HandleFunc("DELETE /api/endpoints/{id}", h.DeleteEndpoint)
	mux.HandleFunc("GET /api/monitor-sse", h.MonitorSSE)
}
