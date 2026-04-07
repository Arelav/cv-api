package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLighthouseInvalidate_POSTOnlyRouting(t *testing.T) {
	t.Parallel()

	h := newLighthouseHandler(&http.Client{Timeout: time.Second})
	mux := http.NewServeMux()
	mux.HandleFunc("POST /lighthouse/invalidate", h.handleInvalidate)

	req := httptest.NewRequest(http.MethodGet, "/lighthouse/invalidate", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET: got status %d, want %d Method Not Allowed", w.Code, http.StatusMethodNotAllowed)
	}

	req = httptest.NewRequest(http.MethodPost, "/lighthouse/invalidate", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST: got status %d, want %d", w.Code, http.StatusOK)
	}
	var body map[string]bool
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body["ok"] {
		t.Fatalf("got body %#v, want ok: true", body)
	}
}
