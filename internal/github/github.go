// Package github is a minimal GitHub REST client covering the two calls the
// pull-request changelog mode needs. It uses net/http only, so it can be
// pointed at an httptest server in tests and at GitHub Enterprise in real use.
package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// DefaultAPI is the public GitHub REST endpoint.
const DefaultAPI = "https://api.github.com"

// apiVersion pins the REST API version sent with every request.
const apiVersion = "2022-11-28"

// PullRequest is a pull request reduced to what the changelog needs.
type PullRequest struct {
	Number   int
	Title    string
	URL      string
	Author   string
	IsBot    bool // the author account has type "Bot"
	Labels   []string
	MergedAt time.Time // zero when the pull request was not merged
}

// Client talks to one repository through the GitHub REST API.
type Client struct {
	BaseURL string // e.g. https://api.github.com, no trailing slash needed
	Owner   string
	Name    string
	Token   string // optional; unauthenticated calls get a much lower rate limit
	HTTP    *http.Client
}

// NewClient returns a client for owner/name. An empty baseURL means DefaultAPI.
func NewClient(baseURL, repo, token string) (*Client, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return nil, fmt.Errorf("github repo %q: want owner/name", repo)
	}
	if baseURL == "" {
		baseURL = DefaultAPI
	}
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Owner:   owner,
		Name:    name,
		Token:   token,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

type apiPull struct {
	Number   int        `json:"number"`
	Title    string     `json:"title"`
	HTMLURL  string     `json:"html_url"`
	MergedAt *time.Time `json:"merged_at"`
	User     struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"user"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// PullsForCommit lists the pull requests associated with a commit
// (GET /repos/{owner}/{repo}/commits/{sha}/pulls). sha must be the full hash.
func (c *Client) PullsForCommit(sha string) ([]PullRequest, error) {
	path := fmt.Sprintf("/repos/%s/%s/commits/%s/pulls?per_page=100",
		url.PathEscape(c.Owner), url.PathEscape(c.Name), url.PathEscape(sha))
	var raw []apiPull
	if err := c.get(path, &raw); err != nil {
		return nil, err
	}
	out := make([]PullRequest, 0, len(raw))
	for _, p := range raw {
		pr := PullRequest{
			Number: p.Number,
			Title:  p.Title,
			URL:    p.HTMLURL,
			Author: p.User.Login,
			IsBot:  p.User.Type == "Bot",
		}
		if p.MergedAt != nil {
			pr.MergedAt = *p.MergedAt
		}
		for _, l := range p.Labels {
			pr.Labels = append(pr.Labels, l.Name)
		}
		out = append(out, pr)
	}
	return out, nil
}

// HasMergedBefore reports whether login authored any pull request in the
// repository that was merged before the given time. It uses the issue search
// endpoint, which has its own, lower rate limit.
func (c *Client) HasMergedBefore(login string, before time.Time) (bool, error) {
	q := fmt.Sprintf("repo:%s/%s is:pr is:merged author:%s merged:<%s",
		c.Owner, c.Name, login, before.UTC().Format(time.RFC3339))
	var res struct {
		TotalCount int `json:"total_count"`
	}
	if err := c.get("/search/issues?per_page=1&q="+url.QueryEscape(q), &res); err != nil {
		return false, err
	}
	return res.TotalCount > 0, nil
}

func (c *Client) get(path string, v any) error {
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("github: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("github: GET %s: %s: %s", path, resp.Status, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("github: decode %s: %w", path, err)
	}
	return nil
}

// remoteRe extracts owner/name from the common GitHub remote URL forms:
// https://host/owner/name(.git), ssh://git@host/owner/name(.git) and
// git@host:owner/name(.git).
var remoteRe = regexp.MustCompile(`^(?:[a-z+]+://(?:[^@/]+@)?[^/]+/|[^@:/]+@[^:]+:)([^/]+)/([^/]+?)(?:\.git)?/?$`)

// RepoFromRemote returns "owner/name" parsed from a git remote URL.
func RepoFromRemote(remote string) (string, bool) {
	m := remoteRe.FindStringSubmatch(strings.TrimSpace(remote))
	if m == nil {
		return "", false
	}
	return m[1] + "/" + m[2], true
}
