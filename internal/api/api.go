// Package api espone le operazioni client-facing del registry: Register,
// Deregister, Discover/List.
//
// Vedi ARCHITETTURA.md, sezione 8.
package api

import (
	"encoding/json"
	"net/http"

	"sdcc-b1-registry/internal/registry"
)

type registerRequest struct {
	ServiceID string            `json:"service_id"`
	Endpoint  string            `json:"endpoint"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type versionResponse struct {
	Counter     uint64 `json:"counter"`
	OwnerNodeID string `json:"owner_node_id"`
}

type serviceResponse struct {
	ServiceID string            `json:"service_id"`
	Endpoint  string            `json:"endpoint"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Version   versionResponse   `json:"version"`
}

func toResponse(e registry.ServiceEntry) serviceResponse {
	return serviceResponse{
		ServiceID: e.ServiceID,
		Endpoint:  e.Endpoint,
		Metadata:  e.Metadata,
		Version: versionResponse{
			Counter:     e.Version.Counter,
			OwnerNodeID: e.Version.OwnerNodeID,
		},
	}
}

// NewMux costruisce l'HTTP mux con le API client-facing del registry,
// appoggiate allo stato locale reg. Un client puo' contattare qualsiasi
// nodo: non c'e' un entry point fisso (ARCHITETTURA.md, sezione 8).
func NewMux(reg *registry.Registry) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /services", handleRegister(reg))
	mux.HandleFunc("DELETE /services/{id}", handleDeregister(reg))
	mux.HandleFunc("GET /services/{id}", handleDiscover(reg))
	mux.HandleFunc("GET /services", handleList(reg))
	return mux
}

func handleRegister(reg *registry.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.ServiceID == "" || req.Endpoint == "" {
			http.Error(w, "service_id and endpoint are required", http.StatusBadRequest)
			return
		}

		entry := reg.Register(req.ServiceID, req.Endpoint, req.Metadata)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(toResponse(entry))
	}
}

func handleDeregister(reg *registry.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if _, ok := reg.Deregister(id); !ok {
			http.Error(w, "service not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleDiscover(reg *registry.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		entry, ok := reg.Discover(id)
		if !ok {
			http.Error(w, "service not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(toResponse(entry))
	}
}

func handleList(reg *registry.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries := reg.List()
		responses := make([]serviceResponse, len(entries))
		for i, e := range entries {
			responses[i] = toResponse(e)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responses)
	}
}
