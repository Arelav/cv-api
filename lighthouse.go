package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
)

type lighthouseHandler struct {
	client  *http.Client
	apiKey  string
	siteURL string
	baseURL string
	cache   *cache[lighthouseResult]
	fetchMu sync.Mutex
}

type lighthouseResult struct {
	Performance   float64           `json:"performance"`
	Accessibility float64           `json:"accessibility"`
	BestPractices float64           `json:"bestPractices"`
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
	Score float64 `json:"score"` // 0.0–1.0
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

func newLighthouseHandler(client *http.Client) *lighthouseHandler {
	return &lighthouseHandler{
		client:  client,
		apiKey:  strings.TrimSpace(os.Getenv("PAGESPEED_API_KEY")),
		siteURL: strings.TrimSpace(os.Getenv("LIGHTHOUSE_URL")),
		baseURL: "https://www.googleapis.com/pagespeedonline/v5/runPagespeed",
		cache:   newCache[lighthouseResult](24 * time.Hour),
	}
}

func (h *lighthouseHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.siteURL == "" {
		const msg = "LIGHTHOUSE_URL is not configured (see .env.example)."
		if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
			hub.CaptureMessage(msg)
		}
		writeAPIError(w, http.StatusInternalServerError, "config", msg)
		return
	}

	result, ok := h.cache.get()
	if !ok {
		h.fetchMu.Lock()
		defer h.fetchMu.Unlock()

		if cached, hit := h.cache.get(); hit {
			writeJSON(w, http.StatusOK, cached, 86400, 604800)
			return
		}

		var err error
		result, err = h.fetch(r.Context())
		if err != nil {
			log.Printf("lighthouse: %v", err)
			const msg = "Could not fetch scores from PageSpeed Insights. Try again later."
			if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
				hub.CaptureException(err)
			}
			writeAPIError(w, http.StatusBadGateway, "upstream", msg)
			return
		}
		h.cache.set(result)
	}

	writeJSON(w, http.StatusOK, result, 86400, 604800)
}

func (h *lighthouseHandler) fetch(ctx context.Context) (lighthouseResult, error) {
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

	reqURL := fmt.Sprintf("%s?%s", h.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return lighthouseResult{}, err
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return lighthouseResult{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return lighthouseResult{}, err
	}

	if resp.StatusCode != http.StatusOK {
		snippet := string(body)
		if len(snippet) > 512 {
			snippet = snippet[:512] + "…"
		}
		log.Printf("lighthouse: PageSpeed %s body=%s", resp.Status, snippet)
		return lighthouseResult{}, fmt.Errorf("PageSpeed API: %s", resp.Status)
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
