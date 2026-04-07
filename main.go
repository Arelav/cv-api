package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
)

// outboundHTTPTimeout caps upstream GitHub and PageSpeed requests.
// WriteTimeout must exceed this so the handler can still serialize and flush
// a response (e.g. 502) after the client hits its deadline.
const (
	outboundHTTPTimeout = 2 * time.Minute
	writeTimeoutMargin  = 10 * time.Second
	serverWriteTimeout  = outboundHTTPTimeout + writeTimeoutMargin
)

func main() {
	if err := sentry.Init(sentry.ClientOptions{
		Dsn: os.Getenv("SENTRY_DSN"),
	}); err != nil {
		log.Fatalf("sentry.Init: %v", err)
	}
	defer sentry.Flush(2 * time.Second)

	outbound := &http.Client{Timeout: outboundHTTPTimeout}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.Handle("GET /github/stats", newGitHubHandler(outbound))
	lh := newLighthouseHandler(outbound)
	mux.Handle("GET /lighthouse", lh)
	mux.HandleFunc("POST /lighthouse/invalidate", lh.handleInvalidate)
	handler := sentryhttp.New(sentryhttp.Options{}).Handle(mux)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      serverWriteTimeout,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
