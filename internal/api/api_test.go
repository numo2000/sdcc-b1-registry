package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sdcc-b1-registry/internal/registry"
)

func TestRegisterCreatesService(t *testing.T) {
	reg := registry.New("node-1")
	mux := NewMux(reg)

	body := bytes.NewBufferString(`{"service_id":"svc-a","endpoint":"10.0.0.1:8080"}`)
	req := httptest.NewRequest(http.MethodPost, "/services", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, ok := reg.Discover("svc-a"); !ok {
		t.Errorf("expected svc-a to be registered in the underlying registry")
	}
}

func TestRegisterRejectsMissingFields(t *testing.T) {
	reg := registry.New("node-1")
	mux := NewMux(reg)

	req := httptest.NewRequest(http.MethodPost, "/services", bytes.NewBufferString(`{"service_id":"svc-a"}`))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing endpoint, got %d", rec.Code)
	}
}

func TestDiscoverReturnsService(t *testing.T) {
	reg := registry.New("node-1")
	reg.Register("svc-a", "10.0.0.1:8080", map[string]string{"zone": "eu"})
	mux := NewMux(reg)

	req := httptest.NewRequest(http.MethodGet, "/services/svc-a", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp serviceResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp.Endpoint != "10.0.0.1:8080" || resp.Metadata["zone"] != "eu" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestDiscoverUnknownServiceReturns404(t *testing.T) {
	reg := registry.New("node-1")
	mux := NewMux(reg)

	req := httptest.NewRequest(http.MethodGet, "/services/does-not-exist", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestDeregisterRemovesService(t *testing.T) {
	reg := registry.New("node-1")
	reg.Register("svc-a", "10.0.0.1:8080", nil)
	mux := NewMux(reg)

	req := httptest.NewRequest(http.MethodDelete, "/services/svc-a", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if _, ok := reg.Discover("svc-a"); ok {
		t.Errorf("expected svc-a to be removed")
	}
}

func TestDeregisterUnknownServiceReturns404(t *testing.T) {
	reg := registry.New("node-1")
	mux := NewMux(reg)

	req := httptest.NewRequest(http.MethodDelete, "/services/does-not-exist", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestListReturnsAllActiveServices(t *testing.T) {
	reg := registry.New("node-1")
	reg.Register("svc-a", "10.0.0.1:8080", nil)
	reg.Register("svc-b", "10.0.0.2:9090", nil)
	reg.Deregister("svc-b")
	mux := NewMux(reg)

	req := httptest.NewRequest(http.MethodGet, "/services", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var resp []serviceResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if len(resp) != 1 || resp[0].ServiceID != "svc-a" {
		t.Errorf("expected only svc-a to be listed, got %+v", resp)
	}
}
