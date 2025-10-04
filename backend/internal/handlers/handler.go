package handlers

import "net/http"

type Handler interface {
	GetEndpoints(w http.ResponseWriter, r *http.Request)
	PostEndpoint(w http.ResponseWriter, r *http.Request)
	PatchEndpoint(w http.ResponseWriter, r *http.Request)
	DeleteEndpoint(w http.ResponseWriter, r *http.Request)
	MonitorSSE(w http.ResponseWriter, r *http.Request)
}
