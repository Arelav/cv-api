package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// upstreamError carries a safe client message and HTTP status for PageSpeed failures.
type upstreamError struct {
	status     int
	code       string
	message    string
	detail     string // optional: upstream body (e.g. Google) — surfaced in JSON as "detail"
	logMessage string
}

func (e *upstreamError) Error() string {
	return e.logMessage
}

type lighthouseHandler struct {
	apiKey  string
	siteURL string
	baseURL string
	cache   *cache[lighthouseResult]
}

type lighthouseResult struct {
	Performance   float64           `json:"performance"`
	Accessibility float64           `json:"accessibility"`
	BestPractices float64           `json:"best_practices"`
	SEO           float64           `json:"seo"`
	Metrics       lighthouseMetrics `json:"metrics"`
}

type lighthouseMetrics struct {
	FCP float64 `json:"fcp"` // First Contentful Paint (ms)
	LCP float64 `json:"lcp"` // Largest Contentful Paint (ms)
	TBT float64 `json:"tbt"` // Total Blocking Time (ms)
	CLS float64 `json:"cls"` // Cumulative Layout Shift (unitless)
	TTI float64 `json:"tti"` // Time to Interactive (ms)
}

type psiCategory struct {
	Score float64 `json:"score"`
}

type psiAudit struct {
	NumericValue float64 `json:"numericValue"`
}

type psiResponse struct {
	LighthouseResult struct {
		Categories struct {
			Performance   psiCategory `json:"performance"`
			Accessibility psiCategory `json:"accessibility"`
			BestPractices psiCategory `json:"best-practices"`
			SEO           psiCategory `json:"seo"`
		} `json:"categories"`
		Audits struct {
			FCP psiAudit `json:"first-contentful-paint"`
			LCP psiAudit `json:"largest-contentful-paint"`
			TBT psiAudit `json:"total-blocking-time"`
			CLS psiAudit `json:"cumulative-layout-shift"`
			TTI psiAudit `json:"interactive"`
		} `json:"audits"`
	} `json:"lighthouseResult"`
}

func newLighthouseHandler() *lighthouseHandler {
	return &lighthouseHandler{
		apiKey:  os.Getenv("PAGESPEED_API_KEY"),
		siteURL: os.Getenv("SITE_URL"),
		baseURL: "https://www.googleapis.com/pagespeedonline/v5/runPagespeed",
		cache:   newCache[lighthouseResult](24 * time.Hour),
	}
}

func (h *lighthouseHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.siteURL == "" {
		writeAPIError(w, http.StatusInternalServerError, "config", "SITE_URL is not configured")
		return
	}

	result, ok := h.cache.get()
	if !ok {
		var err error
		result, err = h.fetch()
		if err != nil {
			var ue *upstreamError
			if errors.As(err, &ue) {
				log.Printf("lighthouse: %s", ue.logMessage)
				writeAPIErrorDetail(w, ue.status, ue.code, ue.message, ue.detail)
				return
			}
			log.Printf("lighthouse: %v", err)
			writeAPIError(w, http.StatusBadGateway, "upstream", "Could not fetch scores from PageSpeed Insights. Try again later.")
			return
		}
		h.cache.set(result)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=86400, stale-while-revalidate=604800")
	json.NewEncoder(w).Encode(result)
}

func (h *lighthouseHandler) fetch() (lighthouseResult, error) {
	const maxAttempts = 4
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// PageSpeed often fails once on cold/slow origins; backoff before retry.
			time.Sleep(time.Duration(attempt) * 3 * time.Second)
		}
		result, err := h.fetchOnce()
		if err == nil {
			return result, nil
		}
		lastErr = err
		var ue *upstreamError
		if errors.As(err, &ue) && pageSpeedIsDocumentLoadFailure(ue.detail) {
			log.Printf("lighthouse: retry after document load failure (attempt %d/%d)", attempt+1, maxAttempts)
			continue
		}
		return lighthouseResult{}, err
	}
	return lighthouseResult{}, lastErr
}

func (h *lighthouseHandler) fetchOnce() (lighthouseResult, error) {
	params := url.Values{}
	params.Set("url", h.siteURL)
	params.Set("strategy", "mobile")
	params.Add("category", "performance")
	params.Add("category", "accessibility")
	params.Add("category", "best-practices")
	params.Add("category", "seo")
	if h.apiKey != "" {
		params.Set("key", h.apiKey)
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s?%s", h.baseURL, params.Encode()), nil)
	if err != nil {
		return lighthouseResult{}, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return lighthouseResult{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return lighthouseResult{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return lighthouseResult{}, pageSpeedHTTPError(resp.StatusCode, body)
	}

	var psi psiResponse
	if err := json.Unmarshal(body, &psi); err != nil {
		return lighthouseResult{}, err
	}

	c := psi.LighthouseResult.Categories
	a := psi.LighthouseResult.Audits

	return lighthouseResult{
		Performance:   c.Performance.Score * 100,
		Accessibility: c.Accessibility.Score * 100,
		BestPractices: c.BestPractices.Score * 100,
		SEO:           c.SEO.Score * 100,
		Metrics: lighthouseMetrics{
			FCP: a.FCP.NumericValue,
			LCP: a.LCP.NumericValue,
			TBT: a.TBT.NumericValue,
			CLS: a.CLS.NumericValue,
			TTI: a.TTI.NumericValue,
		},
	}, nil
}

// googleAPIErrorBody matches errors returned by googleapis JSON endpoints.
type googleAPIErrorBody struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func pageSpeedHTTPError(httpStatus int, body []byte) error {
	var apiErr googleAPIErrorBody
	_ = json.Unmarshal(body, &apiErr)
	detail := apiErr.Error.Message
	if detail == "" {
		detail = string(body)
		if len(detail) > 600 {
			detail = detail[:600] + "…"
		}
	}
	logLine := fmt.Sprintf("PageSpeed HTTP %d: %s", httpStatus, detail)

	if pageSpeedIsDocumentLoadFailure(detail) {
		return &upstreamError{
			status: http.StatusBadGateway,
			code:   "document_request",
			message: "PageSpeed could not load your URL from Google's network (timeout or failed fetch). " +
				"This is not an API key problem. Check SITE_URL is the correct public https URL and that the page responds quickly; the API retries automatically on transient timeouts.",
			detail:     detail,
			logMessage: logLine,
		}
	}

	switch httpStatus {
	case http.StatusTooManyRequests: // 429
		return &upstreamError{
			status:     http.StatusServiceUnavailable,
			code:       "quota",
			message:    "PageSpeed Insights quota exceeded. Create a Google Cloud API key with the PageSpeed Insights API enabled, set PAGESPEED_API_KEY on the server, and ensure billing/quota for the project.",
			detail:     detail,
			logMessage: logLine,
		}
	case http.StatusForbidden, http.StatusBadRequest:
		return &upstreamError{
			status:     http.StatusBadGateway,
			code:       "upstream",
			message:    "PageSpeed API rejected the request. Verify PAGESPEED_API_KEY and that the PageSpeed Insights API is enabled for that key.",
			detail:     detail,
			logMessage: logLine,
		}
	default:
		return &upstreamError{
			status:     http.StatusBadGateway,
			code:       "upstream",
			message:    "Could not fetch scores from PageSpeed Insights. Try again later.",
			detail:     detail,
			logMessage: logLine,
		}
	}
}

// pageSpeedIsDocumentLoadFailure detects Lighthouse load errors (not auth/quota).
func pageSpeedIsDocumentLoadFailure(detail string) bool {
	d := strings.ToLower(detail)
	return strings.Contains(d, "failed_document_request") ||
		strings.Contains(d, "err_timed_out") ||
		strings.Contains(d, "err_connection") ||
		strings.Contains(d, "err_connection_refused") ||
		strings.Contains(d, "dns_probe")
}
