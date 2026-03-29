package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestGitHubServer(t *testing.T, user ghUser, repos []ghRepo) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/users/"+user.Login, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	})

	mux.HandleFunc("/users/"+user.Login+"/repos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repos)
	})

	return httptest.NewServer(mux)
}

func TestGitHubHandler_NoUsername(t *testing.T) {
	h := &githubHandler{client: http.DefaultClient, cache: newCache[githubStats](time.Hour)}

	req := httptest.NewRequest(http.MethodGet, "/github/stats", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestGitHubHandler_ReturnsStats(t *testing.T) {
	user := ghUser{Login: "testuser", Name: "Test User", PublicRepos: 3, Followers: 10}
	repos := []ghRepo{
		{StargazersCount: 5, Language: "Go", Fork: false},
		{StargazersCount: 2, Language: "Go", Fork: false},
		{StargazersCount: 1, Language: "TypeScript", Fork: false},
		{StargazersCount: 99, Language: "Python", Fork: true}, // fork — excluded
	}

	srv := newTestGitHubServer(t, user, repos)
	defer srv.Close()

	h := &githubHandler{
		client:   http.DefaultClient,
		username: user.Login,
		baseURL:  srv.URL,
		cache:    newCache[githubStats](time.Hour),
	}

	req := httptest.NewRequest(http.MethodGet, "/github/stats", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want %d", w.Code, http.StatusOK)
	}

	var stats githubStats
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if stats.Username != "testuser" {
		t.Errorf("username: got %q, want %q", stats.Username, "testuser")
	}
	if stats.TotalStars != 8 { // 5+2+1, fork excluded
		t.Errorf("total_stars: got %d, want 8", stats.TotalStars)
	}
	if len(stats.Languages) == 0 || stats.Languages[0].Name != "Go" {
		t.Errorf("top language: got %+v, want Go first", stats.Languages)
	}
}

func TestGitHubHandler_UsesCache(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/users/testuser" {
			json.NewEncoder(w).Encode(ghUser{Login: "testuser"})
		} else {
			json.NewEncoder(w).Encode([]ghRepo{})
		}
	}))
	defer srv.Close()

	h := &githubHandler{
		client:   http.DefaultClient,
		username: "testuser",
		baseURL:  srv.URL,
		cache:    newCache[githubStats](time.Hour),
	}

	for range 3 {
		req := httptest.NewRequest(http.MethodGet, "/github/stats", nil)
		h.ServeHTTP(httptest.NewRecorder(), req)
	}

	if calls > 2 { // user + repos on first request only
		t.Errorf("expected ≤2 API calls, got %d", calls)
	}
}
