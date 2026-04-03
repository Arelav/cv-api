package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
)

type githubHandler struct {
	client            *http.Client
	token             string
	username          string
	baseURL           string
	graphqlURL        string
	maxAgeSeconds     int
	staleWhileSeconds int
	cache             *cache[githubStats]
	fetchMu           sync.Mutex
}

type githubStats struct {
	Username    string         `json:"username"`
	Name        string         `json:"name"`
	PublicRepos int            `json:"publicRepos"`
	Followers   int            `json:"followers"`
	TotalStars  int            `json:"totalStars"`
	Languages   []languageStat `json:"topLanguages"`
	TopRepos    []repoStat     `json:"topRepos"`
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
		client:            client,
		token:             strings.TrimSpace(os.Getenv("GITHUB_TOKEN")),
		username:          strings.TrimSpace(os.Getenv("GITHUB_USERNAME")),
		baseURL:           "https://api.github.com",
		graphqlURL:        "https://api.github.com/graphql",
		maxAgeSeconds:     int(ttl.Seconds()),
		staleWhileSeconds: int((24 * time.Hour).Seconds()),
		cache:             newCache[githubStats](ttl),
	}
}

func (h *githubHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.username == "" {
		const msg = "GITHUB_USERNAME is not configured"
		if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
			hub.CaptureMessage(msg)
		}
		writeAPIError(w, http.StatusInternalServerError, "config", msg)
		return
	}

	if stats, ok := h.cache.get(); ok {
		writeJSON(w, http.StatusOK, stats, h.maxAgeSeconds, h.staleWhileSeconds)
		return
	}

	h.fetchMu.Lock()
	defer h.fetchMu.Unlock()

	if stats, ok := h.cache.get(); ok {
		writeJSON(w, http.StatusOK, stats, h.maxAgeSeconds, h.staleWhileSeconds)
		return
	}

	stats, err := h.fetch(r.Context())
	if err != nil {
		log.Printf("github: %v", err)
		const msg = "Could not fetch GitHub stats. Try again later."
		if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
			hub.CaptureException(err)
		}
		writeAPIError(w, http.StatusBadGateway, "upstream", msg)
		return
	}

	h.cache.set(stats)
	writeJSON(w, http.StatusOK, stats, h.maxAgeSeconds, h.staleWhileSeconds)
}

func (h *githubHandler) fetch(ctx context.Context) (githubStats, error) {
	user, err := h.getUser(ctx)
	if err != nil {
		return githubStats{}, fmt.Errorf("get user: %w", err)
	}

	repos, err := h.getRepos(ctx)
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
	topRepos := topReposByStars(ownRepos)
	if h.token != "" {
		pinned, err := h.getPinnedRepos(ctx)
		if err == nil && len(pinned) > 0 {
			topRepos = pinned
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

func (h *githubHandler) getUser(ctx context.Context) (ghUser, error) {
	var user ghUser
	err := h.githubGET(ctx, fmt.Sprintf("%s/users/%s", h.baseURL, h.username), &user)
	return user, err
}

func topReposByStars(ownRepos []ghRepo) []repoStat {
	sort.Slice(ownRepos, func(i, j int) bool {
		return ownRepos[i].StargazersCount > ownRepos[j].StargazersCount
	})
	if len(ownRepos) > 6 {
		ownRepos = ownRepos[:6]
	}
	out := make([]repoStat, len(ownRepos))
	for i, r := range ownRepos {
		out[i] = repoStat{
			Name:        r.Name,
			Description: r.Description,
			Stars:       r.StargazersCount,
			URL:         r.HTMLURL,
			Language:    r.Language,
		}
	}
	return out
}

type gqlPinnedResponse struct {
	Data *struct {
		User *struct {
			PinnedItems struct {
				Nodes []struct {
					Name            string  `json:"name"`
					Description     *string `json:"description"`
					StargazerCount  int     `json:"stargazerCount"`
					URL             string  `json:"url"`
					PrimaryLanguage *struct {
						Name string `json:"name"`
					} `json:"primaryLanguage"`
				} `json:"nodes"`
			} `json:"pinnedItems"`
		} `json:"user"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (h *githubHandler) getPinnedRepos(ctx context.Context) ([]repoStat, error) {
	const q = `query($login: String!) {
		user(login: $login) {
			pinnedItems(first: 6, types: REPOSITORY) {
				nodes {
					... on Repository {
						name
						description
						stargazerCount
						url
						primaryLanguage { name }
					}
				}
			}
		}
	}`
	body, err := json.Marshal(map[string]any{
		"query": q,
		"variables": map[string]string{
			"login": h.username,
		},
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.graphqlURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub GraphQL: %s", resp.Status)
	}

	var out gqlPinnedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Errors) > 0 {
		return nil, fmt.Errorf("graphql: %s", out.Errors[0].Message)
	}
	if out.Data == nil || out.Data.User == nil {
		return nil, fmt.Errorf("graphql: missing user")
	}
	nodes := out.Data.User.PinnedItems.Nodes
	repoStats := make([]repoStat, 0, len(nodes))
	for _, n := range nodes {
		desc := ""
		if n.Description != nil {
			desc = *n.Description
		}
		lang := ""
		if n.PrimaryLanguage != nil {
			lang = n.PrimaryLanguage.Name
		}
		repoStats = append(repoStats, repoStat{
			Name:        n.Name,
			Description: desc,
			Stars:       n.StargazerCount,
			URL:         n.URL,
			Language:    lang,
		})
	}
	return repoStats, nil
}

func (h *githubHandler) getRepos(ctx context.Context) ([]ghRepo, error) {
	var all []ghRepo
	for page := 1; ; page++ {
		var batch []ghRepo
		url := fmt.Sprintf("%s/users/%s/repos?per_page=100&page=%d", h.baseURL, h.username, page)
		if err := h.githubGET(ctx, url, &batch); err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) < 100 {
			break
		}
	}
	return all, nil
}

func (h *githubHandler) githubGET(ctx context.Context, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
