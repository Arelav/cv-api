package main

import (
	"encoding/json"
	"net/http"
)

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeAPIErrorDetail(w, status, code, message, "")
}

// writeAPIErrorDetail adds an optional detail string (e.g. upstream provider message) for debugging.
func writeAPIErrorDetail(w http.ResponseWriter, status int, code, message, detail string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	out := map[string]string{"error": code, "message": message}
	if detail != "" {
		out["detail"] = detail
	}
	_ = json.NewEncoder(w).Encode(out)
}
