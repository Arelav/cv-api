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
	client   *http.Client
	token    string
	username string
	baseURL  string
	cache    *cache[githubStats]
}

type githubStats struct {
	Username    string         `json:"username"`
	Name        string         `json:"name"`
	PublicRepos int            `json:"public_repos"`
	Followers   int            `json:"followers"`
	TotalStars  int            `json:"total_stars"`
	Languages   []languageStat `json:"top_languages"`
	TopRepos    []repoStat     `json:"top_repos"`
}

type languageStat struct {
	Name  string `json:"name"`
	Repos int    `json:"repos"`
}

type repoStat struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Stars       int    `json:"stars"`
	URL         string `json:"url"`
	Language    string `json:"language"`
}

type ghUser struct {
	Login       string `json:"login"`
	Name        string `json:"name"`
	PublicRepos int    `json:"public_repos"`
	Followers   int    `json:"followers"`
}

type ghRepo struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	StargazersCount int    `json:"stargazers_count"`
	Language        string `json:"language"`
	HTMLURL         string `json:"html_url"`
	Fork            bool   `json:"fork"`
}

func newGitHubHandler(client *http.Client) *githubHandler {
	ttl := time.Hour
	if s := os.Getenv("GITHUB_CACHE_TTL"); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			ttl = d
		}
	}
	return &githubHandler{
		client:   client,
		token:    os.Getenv("GITHUB_TOKEN"),
		username: os.Getenv("GITHUB_USERNAME"),
		baseURL:  "https://api.github.com",
		cache:    newCache[githubStats](ttl),
	}
}

func (h *githubHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.username == "" {
		writeAPIError(w, http.StatusInternalServerError, "config", "GITHUB_USERNAME is not configured")
		return
	}

	if stats, ok := h.cache.get(); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600, stale-while-revalidate=86400")
		json.NewEncoder(w).Encode(stats)
		return
	}

	stats, err := h.fetch()
	if err != nil {
		log.Printf("github: %v", err)
		writeAPIError(w, http.StatusBadGateway, "upstream", "Could not fetch GitHub stats. Try again later.")
		return
	}

	h.cache.set(stats)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600, stale-while-revalidate=86400")
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

	ownRepos := make([]ghRepo, 0, len(repos))
	for _, r := range repos {
		if !r.Fork {
			ownRepos = append(ownRepos, r)
		}
	}
	sort.Slice(ownRepos, func(i, j int) bool {
		return ownRepos[i].StargazersCount > ownRepos[j].StargazersCount
	})
	if len(ownRepos) > 6 {
		ownRepos = ownRepos[:6]
	}
	topRepos := make([]repoStat, len(ownRepos))
	for i, r := range ownRepos {
		topRepos[i] = repoStat{
			Name:        r.Name,
			Description: r.Description,
			Stars:       r.StargazersCount,
			URL:         r.HTMLURL,
			Language:    r.Language,
		}
	}

	return githubStats{
		Username:    user.Login,
		Name:        user.Name,
		PublicRepos: user.PublicRepos,
		Followers:   user.Followers,
		TotalStars:  totalStars,
		Languages:   langs,
		TopRepos:    topRepos,
	}, nil
}

func (h *githubHandler) getUser() (ghUser, error) {
	var user ghUser
	err := h.githubGET(fmt.Sprintf("%s/users/%s", h.baseURL, h.username), &user)
	return user, err
}

func (h *githubHandler) getRepos() ([]ghRepo, error) {
	var all []ghRepo
	for page := 1; ; page++ {
		var batch []ghRepo
		url := fmt.Sprintf("%s/users/%s/repos?per_page=100&page=%d", h.baseURL, h.username, page)
		if err := h.githubGET(url, &batch); err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) < 100 {
			break
		}
	}
	return all, nil
}

func (h *githubHandler) githubGET(url string, v any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if h.token != "" {
		req.Header.Set("Authorization", "Bearer "+h.token)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API %s: %s", url, resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(v)
}
