package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "message": message})
}

func writeJSON(w http.ResponseWriter, status int, v any, maxAgeSeconds, staleWhileSeconds int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set(
		"Cache-Control",
		"public, max-age="+strconv.Itoa(maxAgeSeconds)+", stale-while-revalidate="+strconv.Itoa(staleWhileSeconds),
	)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
