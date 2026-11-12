package github

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPullsForCommit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/tool/commits/abc123/pulls" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer s3cret" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("X-GitHub-Api-Version"); got != apiVersion {
			t.Errorf("X-GitHub-Api-Version = %q", got)
		}
		w.Write([]byte(`[
			{"number": 12, "title": "feat: add x", "html_url": "https://github.com/acme/tool/pull/12",
			 "merged_at": "2026-07-20T10:00:00Z", "user": {"login": "alice", "type": "User"},
			 "labels": [{"name": "breaking-change"}]},
			{"number": 13, "title": "wip", "merged_at": null,
			 "user": {"login": "dependabot[bot]", "type": "Bot"}, "labels": []}
		]`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL+"/", "acme/tool", "s3cret")
	if err != nil {
		t.Fatal(err)
	}
	prs, err := c.PullsForCommit("abc123")
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 2 {
		t.Fatalf("got %d pull requests, want 2", len(prs))
	}
	p := prs[0]
	if p.Number != 12 || p.Author != "alice" || p.IsBot || len(p.Labels) != 1 || p.Labels[0] != "breaking-change" {
		t.Errorf("first PR = %+v", p)
	}
	if !p.MergedAt.Equal(time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("merged_at = %v", p.MergedAt)
	}
	if !prs[1].MergedAt.IsZero() || !prs[1].IsBot {
		t.Errorf("second PR should be unmerged and a bot: %+v", prs[1])
	}
}

func TestHasMergedBefore(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/issues" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("no token was configured, Authorization must be empty")
		}
		query = r.URL.Query().Get("q")
		if strings.Contains(query, "author:bob") {
			w.Write([]byte(`{"total_count": 0}`))
			return
		}
		w.Write([]byte(`{"total_count": 4}`))
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "acme/tool", "")
	at := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)

	got, err := c.HasMergedBefore("alice", at)
	if err != nil || !got {
		t.Fatalf("alice: got %v, %v; want true", got, err)
	}
	want := "repo:acme/tool is:pr is:merged author:alice merged:<2026-07-20T10:00:00Z"
	if query != want {
		t.Errorf("q = %q, want %q", query, want)
	}
	if got, _ := c.HasMergedBefore("bob", at); got {
		t.Error("bob has no earlier merged PRs")
	}
}

func TestErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "acme/tool", "bad")
	_, err := c.PullsForCommit("abc")
	if err == nil || !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "Bad credentials") {
		t.Errorf("err = %v, want a 401 with the API message", err)
	}
}

func TestNewClientRejectsBadRepo(t *testing.T) {
	for _, r := range []string{"", "acme", "/tool", "acme/", "acme/tool/extra"} {
		if _, err := NewClient("", r, ""); err == nil {
			t.Errorf("NewClient(%q) should fail", r)
		}
	}
	c, err := NewClient("", "acme/tool", "")
	if err != nil || c.BaseURL != DefaultAPI {
		t.Errorf("default base URL: %v %v", c, err)
	}
}

func TestRepoFromRemote(t *testing.T) {
	cases := map[string]string{
		"https://github.com/acme/tool.git":                  "acme/tool",
		"https://github.com/acme/tool":                      "acme/tool",
		"https://x-access-token:t@github.com/acme/tool.git": "acme/tool",
		"git@github.com:acme/tool.git":                      "acme/tool",
		"ssh://git@github.com/acme/tool.git":                "acme/tool",
		"https://ghe.example.com/acme/tool.js/":             "acme/tool.js",
	}
	for in, want := range cases {
		got, ok := RepoFromRemote(in)
		if !ok || got != want {
			t.Errorf("RepoFromRemote(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "/local/path", "https://github.com/acme"} {
		if got, ok := RepoFromRemote(bad); ok {
			t.Errorf("RepoFromRemote(%q) = %q, want no match", bad, got)
		}
	}
}
