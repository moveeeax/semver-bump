// Package bump maps a set of Conventional Commits to a semver bump decision.
package bump

import (
	"github.com/moveeeax/semver-bump/internal/conventional"
	"github.com/moveeeax/semver-bump/internal/semver"
)

// Level is the magnitude of a version bump.
type Level int

const (
	None Level = iota
	Patch
	Minor
	Major
)

func (l Level) String() string {
	switch l {
	case Patch:
		return "patch"
	case Minor:
		return "minor"
	case Major:
		return "major"
	default:
		return "none"
	}
}

// patchTypes are commit types that, on their own, warrant a patch release.
var patchTypes = map[string]bool{
	"fix":      true,
	"perf":     true,
	"revert":   true,
	"refactor": true,
}

// LevelFor returns the highest bump level implied by the commits: any breaking
// change wins (major), else any feature (minor), else any fix/perf (patch).
func LevelFor(commits []conventional.Commit) Level {
	level := None
	for _, c := range commits {
		switch {
		case c.Breaking:
			return Major
		case c.Type == "feat":
			if level < Minor {
				level = Minor
			}
		case patchTypes[c.Type]:
			if level < Patch {
				level = Patch
			}
		}
	}
	return level
}

// Next applies a bump level to a version. For 0.x versions (initial
// development) a breaking change is downgraded to a minor bump, so the tool
// never promotes an unstable project to 1.0.0 on its own — the semantic-release
// convention. Set allowMajorZero to opt out and get a real major bump.
func Next(cur semver.Version, level Level, allowMajorZero bool) semver.Version {
	if cur.Major == 0 && !allowMajorZero {
		switch level {
		case Major:
			return cur.BumpMinor()
		case Minor:
			return cur.BumpPatch()
		}
	}
	switch level {
	case Major:
		return cur.BumpMajor()
	case Minor:
		return cur.BumpMinor()
	case Patch:
		return cur.BumpPatch()
	default:
		return cur
	}
}
