package release

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/moveeeax/semver-bump/internal/github"
	"github.com/moveeeax/semver-bump/internal/gitrepo"
)

// fakePRs is an in-memory PRSource keyed by commit hash.
type fakePRs struct {
	byCommit map[string][]github.PullRequest
	veterans map[string]bool // logins with pull requests merged before
	searched []string
}

func (f *fakePRs) PullsForCommit(sha string) ([]github.PullRequest, error) {
	return f.byCommit[sha], nil
}

func (f *fakePRs) HasMergedBefore(login string, _ time.Time) (bool, error) {
	f.searched = append(f.searched, login)
	return f.veterans[login], nil
}

func merged(day int) time.Time { return time.Date(2026, 7, day, 12, 0, 0, 0, time.UTC) }

func TestComputeFromPullRequests(t *testing.T) {
	commits := []gitrepo.RawCommit{
		{Hash: "a000000", FullHash: "a-full", Message: "Merge pull request #3 from alice/x"},
		{Hash: "b000000", FullHash: "b-full", Message: "wip on x"},
		{Hash: "c000000", FullHash: "c-full", Message: "fix(io): direct push"},
		{Hash: "d000000", FullHash: "d-full", Message: "Bump deps"},
		{Hash: "e000000", FullHash: "e-full", Message: "first PR from carol"},
	}
	alicePR := github.PullRequest{Number: 3, Title: "feat(cli): add --dry flag", Author: "alice", MergedAt: merged(20)}
	src := &fakePRs{
		byCommit: map[string][]github.PullRequest{
			// Both commits of a merge-commit PR resolve to the same PR.
			"a-full": {alicePR},
			"b-full": {alicePR, {Number: 9, Title: "feat: never merged", Author: "alice"}},
			"d-full": {{Number: 5, Title: "chore(deps): bump x", Author: "dependabot[bot]", IsBot: true, MergedAt: merged(18)}},
			"e-full": {{Number: 4, Title: "Speed up parsing", Author: "carol", Labels: []string{"type: perf"}, MergedAt: merged(19)}},
		},
		veterans: map[string]bool{"alice": true},
	}
	r := &fakeRepo{tag: "v1.2.0", commits: commits}
	p, err := Compute(r, Options{Now: fixedNow, PullRequests: src})
	if err != nil {
		t.Fatal(err)
	}
	if p.PullRequests != 3 {
		t.Errorf("pull requests = %d, want 3 (#3, #4, #5; #9 is unmerged)", p.PullRequests)
	}
	if len(p.Commits) != 4 || p.Skipped != 0 {
		t.Errorf("entries = %d skipped = %d, want 4 and 0", len(p.Commits), p.Skipped)
	}
	if p.NextVersion.String() != "1.3.0" {
		t.Errorf("next = %q, want 1.3.0", p.NextVersion)
	}
	for _, want := range []string{
		"- **cli:** add --dry flag (#3) by @alice\n",
		"- Speed up parsing (#4) by @carol (first contribution)\n",
		"- **io:** direct push (c000000)\n",
	} {
		if !strings.Contains(p.ChangelogSection, want) {
			t.Errorf("changelog missing %q:\n%s", want, p.ChangelogSection)
		}
	}
	if strings.Contains(p.ChangelogSection, "never merged") || strings.Contains(p.ChangelogSection, "dependabot") {
		t.Errorf("unmerged PRs and bot credits must not appear:\n%s", p.ChangelogSection)
	}
	for _, login := range src.searched {
		if login == "dependabot[bot]" {
			t.Error("bots should not be looked up as contributors")
		}
	}
}

func TestPullRequestBreakingLabelAndPattern(t *testing.T) {
	src := &fakePRs{
		byCommit: map[string][]github.PullRequest{
			"x": {{Number: 7, Title: "fix: tighten config", Author: "renovate-bot", Labels: []string{"Breaking-Change"}, MergedAt: merged(21)}},
			"y": {{Number: 8, Title: "Update README", Author: "dave", MergedAt: merged(22)}},
		},
	}
	r := &fakeRepo{tag: "v1.0.0", commits: []gitrepo.RawCommit{
		{Hash: "x", Message: "squash"}, {Hash: "y", Message: "squash"},
	}}
	p, err := Compute(r, Options{Now: fixedNow, PullRequests: src, BotPattern: regexp.MustCompile(`-bot$`)})
	if err != nil {
		t.Fatal(err)
	}
	if p.NextVersion.String() != "2.0.0" {
		t.Errorf("next = %q, want 2.0.0 from the breaking-change label", p.NextVersion)
	}
	if p.Skipped != 1 {
		t.Errorf("skipped = %d, want 1 (non-conventional title, no type label)", p.Skipped)
	}
	if !strings.Contains(p.ChangelogSection, "### ⚠ BREAKING CHANGES\n\n- tighten config (#7)\n") {
		t.Errorf("breaking entry should be listed without a bot credit:\n%s", p.ChangelogSection)
	}
	if len(src.searched) != 0 {
		t.Errorf("no human authors were credited, searched %v", src.searched)
	}
}

func TestFirstContributionMarksEarliestOnly(t *testing.T) {
	src := &fakePRs{byCommit: map[string][]github.PullRequest{
		"late":  {{Number: 11, Title: "feat: second", Author: "erin", MergedAt: merged(25)}},
		"early": {{Number: 10, Title: "fix: first", Author: "erin", MergedAt: merged(23)}},
	}}
	r := &fakeRepo{tag: "v1.0.0", commits: []gitrepo.RawCommit{
		{Hash: "late", Message: "x"}, {Hash: "early", Message: "x"},
	}}
	p, err := Compute(r, Options{Now: fixedNow, PullRequests: src})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.ChangelogSection, "- first (#10) by @erin (first contribution)\n") ||
		!strings.Contains(p.ChangelogSection, "- second (#11) by @erin\n") {
		t.Errorf("only the earliest PR should carry the marker:\n%s", p.ChangelogSection)
	}
	if len(src.searched) != 1 {
		t.Errorf("one search per author expected, got %v", src.searched)
	}
}
