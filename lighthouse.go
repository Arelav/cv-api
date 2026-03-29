package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

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

	resp, err := http.Get(fmt.Sprintf("%s?%s", h.baseURL, params.Encode()))
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
