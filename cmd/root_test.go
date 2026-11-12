package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// newTestRepo creates a repository with a v1.0.0 tag, one squash-merged
// commit after it and an origin remote pointing at acme/tool.
func newTestRepo(t *testing.T) (dir, sha string) {
	t.Helper()
	dir = t.TempDir()
	r, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, _ := r.Worktree()
	sig := &object.Signature{Name: "t", Email: "t@example.com", When: time.Now()}
	commit := func(name, msg string) string {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := wt.Add(name); err != nil {
			t.Fatal(err)
		}
		h, err := wt.Commit(msg, &git.CommitOptions{Author: sig})
		if err != nil {
			t.Fatal(err)
		}
		return h.String()
	}
	first := commit("a", "feat: initial")
	if _, err := r.CreateTag("v1.0.0", plumbing.NewHash(first), nil); err != nil {
		t.Fatal(err)
	}
	sha = commit("b", "Add the thing (#21)")
	if _, err := r.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{"git@github.com:acme/tool.git"}}); err != nil {
		t.Fatal(err)
	}
	return dir, sha
}

func TestRunPullRequestSource(t *testing.T) {
	dir, sha := newTestRepo(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/tool/commits/" + sha + "/pulls":
			w.Write([]byte(`[{"number": 21, "title": "Add the thing", "merged_at": "2026-07-25T09:00:00Z",
				"user": {"login": "alice", "type": "User"}, "labels": [{"name": "type: feat"}]}]`))
		case "/search/issues":
			w.Write([]byte(`{"total_count": 0}`))
		default:
			t.Errorf("unexpected request %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("GITHUB_REPOSITORY", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	var out bytes.Buffer
	c := newRootCmd()
	c.SetOut(&out)
	c.SetArgs([]string{"--repo", dir, "--changelog-source", "prs", "--github-api", srv.URL, "--json"})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("bad JSON %q: %v", out.String(), err)
	}
	if got["next"] != "1.1.0" || got["source"] != "prs" || got["pull_requests"] != float64(1) {
		t.Errorf("unexpected plan: %v", got)
	}
	if cl, _ := got["changelog"].(string); !strings.Contains(cl, "- Add the thing (#21) by @alice (first contribution)") {
		t.Errorf("changelog = %q", cl)
	}
}

func TestRunRejectsUnknownSource(t *testing.T) {
	dir, _ := newTestRepo(t)
	c := newRootCmd()
	c.SetOut(&bytes.Buffer{})
	c.SetArgs([]string{"--repo", dir, "--changelog-source", "tags"})
	if err := c.Execute(); err == nil || !strings.Contains(err.Error(), "want commits or prs") {
		t.Errorf("err = %v", err)
	}
}
