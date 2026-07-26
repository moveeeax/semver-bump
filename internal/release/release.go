// Package release ties the pieces together: read the repo, parse commits,
// decide the next version and render the changelog. It performs no writes, so
// the whole plan can be computed and tested against a fake repository.
package release

import (
	"time"

	"github.com/moveeeax/semver-bump/internal/bump"
	"github.com/moveeeax/semver-bump/internal/changelog"
	"github.com/moveeeax/semver-bump/internal/conventional"
	"github.com/moveeeax/semver-bump/internal/gitrepo"
	"github.com/moveeeax/semver-bump/internal/semver"
)

// Options tune how the next version is computed.
type Options struct {
	Prerelease     string    // e.g. "rc"; appended as "<pre>.1" on a bump
	AllowMajorZero bool      // permit a real 1.0.0 bump out of 0.x
	Now            time.Time // date stamp for the changelog (defaults to today)
}

// Plan is the result of analysing the repository. Nothing is written yet.
type Plan struct {
	PreviousTag      string
	CurrentVersion   semver.Version
	NextVersion      semver.Version
	Level            bump.Level
	Commits          []conventional.Commit
	Skipped          int // non-conventional commits ignored
	ChangelogSection string
	HasChanges       bool
}

// Compute analyses the repository and returns the release plan.
func Compute(r gitrepo.Repo, opt Options) (Plan, error) {
	now := opt.Now
	if now.IsZero() {
		now = time.Now()
	}

	var p Plan
	tag, found, err := r.LatestSemverTag()
	if err != nil {
		return Plan{}, err
	}
	if found {
		p.PreviousTag = tag.Name
		p.CurrentVersion = tag.Version
	} else {
		p.CurrentVersion = semver.Version{} // 0.0.0
	}

	raw, err := r.CommitsSince(p.PreviousTag)
	if err != nil {
		return Plan{}, err
	}
	for _, rc := range raw {
		if c, ok := conventional.ParseWithHash(rc.Hash, rc.Message); ok {
			p.Commits = append(p.Commits, c)
		} else {
			p.Skipped++
		}
	}

	p.Level = bump.LevelFor(p.Commits)
	p.HasChanges = p.Level != bump.None

	next := bump.Next(p.CurrentVersion, p.Level, opt.AllowMajorZero)
	if opt.Prerelease != "" && p.HasChanges {
		next = next.WithPrerelease(opt.Prerelease + ".1")
	}
	p.NextVersion = next

	p.ChangelogSection = changelog.Render(
		p.NextVersion.String(), now.Format("2006-01-02"), p.Commits)
	return p, nil
}
