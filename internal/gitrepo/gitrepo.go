// Package gitrepo isolates all Git side effects behind a small interface so
// the rest of the tool can be tested without a real repository or network.
package gitrepo

import (
	"fmt"
	"sort"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/moveeeax/semver-bump/internal/semver"
)

// RawCommit is a commit reduced to what the tool needs.
type RawCommit struct {
	Hash     string // short hash
	FullHash string // full 40-character hash, used to look up pull requests
	Message  string // full commit message
}

// TagRef pairs a tag's name with its parsed version.
type TagRef struct {
	Name    string
	Version semver.Version
}

// Repo is the surface the CLI depends on. Both a real go-git implementation
// and an in-memory fake satisfy it.
type Repo interface {
	// LatestSemverTag returns the highest semver tag, or ok=false if none.
	LatestSemverTag() (TagRef, bool, error)
	// CommitsSince returns commits reachable from HEAD that are newer than the
	// given tag, most-recent first. An empty tag means the whole history.
	CommitsSince(tag string) ([]RawCommit, error)
	// CreateTag creates an annotated tag at HEAD.
	CreateTag(name, message string) error
}

// GitRepo is the go-git backed implementation of Repo.
type GitRepo struct {
	repo *git.Repository
}

// Open loads the repository rooted at (or above) path.
func Open(path string) (*GitRepo, error) {
	r, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, fmt.Errorf("open git repo at %q: %w", path, err)
	}
	return &GitRepo{repo: r}, nil
}

// LatestSemverTag scans all tags, keeps those that parse as semver, and returns
// the one with the highest precedence.
func (g *GitRepo) LatestSemverTag() (TagRef, bool, error) {
	iter, err := g.repo.Tags()
	if err != nil {
		return TagRef{}, false, err
	}
	var tags []TagRef
	_ = iter.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().Short()
		v, err := semver.Parse(name)
		if err == nil {
			tags = append(tags, TagRef{Name: name, Version: v})
		}
		return nil
	})
	if len(tags) == 0 {
		return TagRef{}, false, nil
	}
	sort.Slice(tags, func(i, j int) bool {
		return semver.Compare(tags[i].Version, tags[j].Version) < 0
	})
	return tags[len(tags)-1], true, nil
}

// CommitsSince walks HEAD's history and stops when it reaches the commit the
// tag points at (exclusive).
func (g *GitRepo) CommitsSince(tag string) ([]RawCommit, error) {
	head, err := g.repo.Head()
	if err != nil {
		return nil, err
	}

	var stop plumbing.Hash
	if tag != "" {
		ref, err := g.repo.Tag(tag)
		if err != nil {
			return nil, fmt.Errorf("resolve tag %q: %w", tag, err)
		}
		if h, err := resolveToCommit(g.repo, ref.Hash()); err == nil {
			stop = h
		}
	}

	iter, err := g.repo.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var out []RawCommit
	err = iter.ForEach(func(c *object.Commit) error {
		if !stop.IsZero() && c.Hash == stop {
			return storer(nil) // stop the walk
		}
		out = append(out, RawCommit{Hash: c.Hash.String()[:7], FullHash: c.Hash.String(), Message: c.Message})
		return nil
	})
	if err != nil && err != errStop {
		return nil, err
	}
	return out, nil
}

// CreateTag adds an annotated tag at HEAD.
func (g *GitRepo) CreateTag(name, message string) error {
	head, err := g.repo.Head()
	if err != nil {
		return err
	}
	_, err = g.repo.CreateTag(name, head.Hash(), &git.CreateTagOptions{
		Message: message,
	})
	return err
}

// RemoteURL returns the first URL configured for the named remote.
func (g *GitRepo) RemoteURL(name string) (string, error) {
	rem, err := g.repo.Remote(name)
	if err != nil {
		return "", fmt.Errorf("remote %q: %w", name, err)
	}
	urls := rem.Config().URLs
	if len(urls) == 0 {
		return "", fmt.Errorf("remote %q has no URL", name)
	}
	return urls[0], nil
}

// errStop unwinds ForEach once the boundary commit is reached.
var errStop = fmt.Errorf("stop iteration")

func storer(error) error { return errStop }

func resolveToCommit(r *git.Repository, h plumbing.Hash) (plumbing.Hash, error) {
	// Annotated tags resolve through a tag object to the commit; lightweight
	// tags already point at the commit.
	if to, err := r.TagObject(h); err == nil {
		if c, err := to.Commit(); err == nil {
			return c.Hash, nil
		}
	}
	return h, nil
}
