package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	outbound := &http.Client{Timeout: 2 * time.Minute}
	gh := newGitHubHandler(outbound)
	lh := newLighthouseHandler(outbound)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.Handle("GET /github/stats", gh)
	mux.Handle("GET /lighthouse", lh)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
