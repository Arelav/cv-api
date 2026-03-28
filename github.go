package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"time"
)

type githubHandler struct {
	token    string
	username string
	cache    *cache[githubStats]
}

type githubStats struct {
	Username    string         `json:"username"`
	Name        string         `json:"name"`
	PublicRepos int            `json:"public_repos"`
	Followers   int            `json:"followers"`
	TotalStars  int            `json:"total_stars"`
	Languages   []languageStat `json:"top_languages"`
}

type languageStat struct {
	Name  string `json:"name"`
	Repos int    `json:"repos"`
}

type ghUser struct {
	Login       string `json:"login"`
	Name        string `json:"name"`
	PublicRepos int    `json:"public_repos"`
	Followers   int    `json:"followers"`
}

type ghRepo struct {
	StargazersCount int    `json:"stargazers_count"`
	Language        string `json:"language"`
	Fork            bool   `json:"fork"`
}

func newGitHubHandler() *githubHandler {
	ttl := time.Hour
	if s := os.Getenv("GITHUB_CACHE_TTL"); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			ttl = d
		}
	}
	return &githubHandler{
		token:    os.Getenv("GITHUB_TOKEN"),
		username: os.Getenv("GITHUB_USERNAME"),
		cache:    newCache[githubStats](ttl),
	}
}

func (h *githubHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.username == "" {
		http.Error(w, "GITHUB_USERNAME not configured", http.StatusInternalServerError)
		return
	}

	if stats, ok := h.cache.get(); ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
		return
	}

	stats, err := h.fetch()
	if err != nil {
		log.Printf("github: %v", err)
		http.Error(w, "failed to fetch GitHub stats", http.StatusBadGateway)
		return
	}

	h.cache.set(stats)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *githubHandler) fetch() (githubStats, error) {
	user, err := h.getUser()
	if err != nil {
		return githubStats{}, fmt.Errorf("get user: %w", err)
	}

	repos, err := h.getRepos()
	if err != nil {
		return githubStats{}, fmt.Errorf("get repos: %w", err)
	}

	totalStars := 0
	langCount := map[string]int{}
	for _, repo := range repos {
		if repo.Fork {
			continue
		}
		totalStars += repo.StargazersCount
		if repo.Language != "" {
			langCount[repo.Language]++
		}
	}

	type kv struct {
		name  string
		count int
	}
	var ranked []kv
	for name, count := range langCount {
		ranked = append(ranked, kv{name, count})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].count > ranked[j].count
	})
	if len(ranked) > 5 {
		ranked = ranked[:5]
	}

	langs := make([]languageStat, len(ranked))
	for i, kv := range ranked {
		langs[i] = languageStat{Name: kv.name, Repos: kv.count}
	}

	return githubStats{
		Username:    user.Login,
		Name:        user.Name,
		PublicRepos: user.PublicRepos,
		Followers:   user.Followers,
		TotalStars:  totalStars,
		Languages:   langs,
	}, nil
}

func (h *githubHandler) getUser() (ghUser, error) {
	var user ghUser
	err := h.get(fmt.Sprintf("https://api.github.com/users/%s", h.username), &user)
	return user, err
}

func (h *githubHandler) getRepos() ([]ghRepo, error) {
	var all []ghRepo
	for page := 1; ; page++ {
		var batch []ghRepo
		url := fmt.Sprintf("https://api.github.com/users/%s/repos?per_page=100&page=%d", h.username, page)
		if err := h.get(url, &batch); err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) < 100 {
			break
		}
	}
	return all, nil
}

func (h *githubHandler) get(url string, v any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API %s: %s", url, resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(v)
}
