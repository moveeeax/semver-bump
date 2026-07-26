package release

import (
	"testing"
	"time"

	"github.com/moveeeax/semver-bump/internal/gitrepo"
	"github.com/moveeeax/semver-bump/internal/semver"
)

// fakeRepo is an in-memory gitrepo.Repo for tests: no git, no network.
type fakeRepo struct {
	tag     string
	commits []gitrepo.RawCommit
	created []string
}

func (f *fakeRepo) LatestSemverTag() (gitrepo.TagRef, bool, error) {
	if f.tag == "" {
		return gitrepo.TagRef{}, false, nil
	}
	return gitrepo.TagRef{Name: f.tag, Version: semver.MustParse(f.tag)}, true, nil
}

func (f *fakeRepo) CommitsSince(string) ([]gitrepo.RawCommit, error) {
	return f.commits, nil
}

func (f *fakeRepo) CreateTag(name, _ string) error {
	f.created = append(f.created, name)
	return nil
}

func rc(msgs ...string) []gitrepo.RawCommit {
	var out []gitrepo.RawCommit
	for i, m := range msgs {
		out = append(out, gitrepo.RawCommit{Hash: string(rune('a'+i)) + "000000", Message: m})
	}
	return out
}

var fixedNow = time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)

func TestComputeMinorFromFeat(t *testing.T) {
	r := &fakeRepo{tag: "v1.2.0", commits: rc("feat: a", "fix: b", "Merge pull request #7")}
	p, err := Compute(r, Options{Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	if p.NextVersion.String() != "1.3.0" {
		t.Errorf("next = %q, want 1.3.0", p.NextVersion)
	}
	if p.Skipped != 1 {
		t.Errorf("skipped = %d, want 1 (the non-conventional merge)", p.Skipped)
	}
	if !p.HasChanges {
		t.Error("expected HasChanges")
	}
}

func TestComputeNoTagStartsFromZero(t *testing.T) {
	r := &fakeRepo{commits: rc("feat: first")}
	p, err := Compute(r, Options{Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	// From 0.0.0 a feat is a minor level, but the 0.x guard makes it a patch.
	if p.NextVersion.String() != "0.0.1" {
		t.Errorf("next = %q, want 0.0.1", p.NextVersion)
	}
}

func TestComputeBreakingWithAllowMajorZero(t *testing.T) {
	r := &fakeRepo{tag: "v0.5.0", commits: rc("feat!: break")}
	p, err := Compute(r, Options{Now: fixedNow, AllowMajorZero: true})
	if err != nil {
		t.Fatal(err)
	}
	if p.NextVersion.String() != "1.0.0" {
		t.Errorf("next = %q, want 1.0.0", p.NextVersion)
	}
}

func TestComputePrerelease(t *testing.T) {
	r := &fakeRepo{tag: "v2.0.0", commits: rc("feat: x")}
	p, err := Compute(r, Options{Now: fixedNow, Prerelease: "rc"})
	if err != nil {
		t.Fatal(err)
	}
	if p.NextVersion.String() != "2.1.0-rc.1" {
		t.Errorf("next = %q, want 2.1.0-rc.1", p.NextVersion)
	}
}

func TestComputeNoRelevantChanges(t *testing.T) {
	r := &fakeRepo{tag: "v1.0.0", commits: rc("chore: deps", "docs: readme")}
	p, err := Compute(r, Options{Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	if p.HasChanges {
		t.Error("chore/docs only should not trigger a release")
	}
	if p.NextVersion.String() != "1.0.0" {
		t.Errorf("next = %q, want unchanged 1.0.0", p.NextVersion)
	}
}
