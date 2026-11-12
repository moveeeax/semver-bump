package release

import (
	"regexp"
	"strings"
	"time"

	"github.com/moveeeax/semver-bump/internal/conventional"
	"github.com/moveeeax/semver-bump/internal/github"
	"github.com/moveeeax/semver-bump/internal/gitrepo"
)

// PRSource looks up pull requests for the PR changelog mode. *github.Client
// satisfies it; tests use an in-memory fake.
type PRSource interface {
	// PullsForCommit returns the pull requests associated with a commit.
	PullsForCommit(sha string) ([]github.PullRequest, error)
	// HasMergedBefore reports whether login had a pull request merged in the
	// repository before the given time.
	HasMergedBefore(login string, before time.Time) (bool, error)
}

// DefaultBotPattern matches GitHub App accounts such as dependabot[bot].
var DefaultBotPattern = regexp.MustCompile(`\[bot\]$`)

// BreakingLabel marks a pull request as a breaking change regardless of title.
const BreakingLabel = "breaking-change"

// typeLabelRe matches a label that overrides the type parsed from the title,
// e.g. "type: fix" or "type:feat".
var typeLabelRe = regexp.MustCompile(`(?i)^type:\s*([a-z]+)$`)

// fromPullRequests turns the commits since the last tag into changelog
// entries keyed by pull request. Each merged pull request contributes one
// entry, however many of the commits belong to it. Commits with no merged pull
// request (direct pushes) are parsed from their message as in commit mode.
func fromPullRequests(src PRSource, raw []gitrepo.RawCommit, bots *regexp.Regexp) (entries []conventional.Commit, prCount, skipped int, err error) {
	if bots == nil {
		bots = DefaultBotPattern
	}
	seen := map[int]bool{}
	merged := map[int]time.Time{} // entry index -> merge time, for credits

	for _, rc := range raw {
		sha := rc.FullHash
		if sha == "" {
			sha = rc.Hash
		}
		pulls, err := src.PullsForCommit(sha)
		if err != nil {
			return nil, 0, 0, err
		}
		found := false
		for _, pr := range pulls {
			if pr.MergedAt.IsZero() {
				continue
			}
			found = true
			if seen[pr.Number] {
				continue
			}
			seen[pr.Number] = true
			prCount++

			c, ok := parsePullRequest(pr)
			if !ok {
				skipped++
				continue
			}
			if !pr.IsBot && !bots.MatchString(pr.Author) {
				c.Author = pr.Author
				merged[len(entries)] = pr.MergedAt
			}
			entries = append(entries, c)
		}
		if found {
			continue
		}
		if c, ok := conventional.ParseWithHash(rc.Hash, rc.Message); ok {
			entries = append(entries, c)
		} else {
			skipped++
		}
	}

	if err := markFirstContributions(src, entries, merged); err != nil {
		return nil, 0, 0, err
	}
	return entries, prCount, skipped, nil
}

// parsePullRequest reads the Conventional Commit header from the title, then
// applies label overrides: a "type: <t>" label replaces the type (and makes a
// non-conforming title usable as the description), and a breaking-change
// label marks the entry breaking.
func parsePullRequest(pr github.PullRequest) (conventional.Commit, bool) {
	c, ok := conventional.Parse(pr.Title)
	for _, l := range pr.Labels {
		if m := typeLabelRe.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
			if !ok {
				c = conventional.Commit{Description: strings.TrimSpace(pr.Title)}
				ok = c.Description != ""
			}
			c.Type = strings.ToLower(m[1])
		}
	}
	if !ok {
		return conventional.Commit{}, false
	}
	for _, l := range pr.Labels {
		if strings.EqualFold(strings.TrimSpace(l), BreakingLabel) {
			c.Breaking = true
		}
	}
	c.PR = pr.Number
	return c, true
}

// markFirstContributions flags, for each credited author, their earliest pull
// request in this release when they had nothing merged before it.
func markFirstContributions(src PRSource, entries []conventional.Commit, merged map[int]time.Time) error {
	earliest := map[string]int{} // author -> entry index
	for i, at := range merged {
		a := entries[i].Author
		if j, ok := earliest[a]; !ok || at.Before(merged[j]) || (at.Equal(merged[j]) && i < j) {
			earliest[a] = i
		}
	}
	for author, i := range earliest {
		before, err := src.HasMergedBefore(author, merged[i])
		if err != nil {
			return err
		}
		entries[i].FirstContribution = !before
	}
	return nil
}
