package changelog

import (
	"strings"
	"testing"

	"github.com/moveeeax/semver-bump/internal/conventional"
)

func parse(msgs ...string) []conventional.Commit {
	var out []conventional.Commit
	for _, m := range msgs {
		if c, ok := conventional.Parse(m); ok {
			out = append(out, c)
		}
	}
	return out
}

func TestRenderGroups(t *testing.T) {
	got := Render("1.4.0", "2026-07-26", parse(
		"feat(cli): add --json flag",
		"fix: correct exit code",
		"feat: second feature",
		"chore: ignored",
	))
	for _, want := range []string{
		"## 1.4.0 - 2026-07-26",
		"### Features",
		"- **cli:** add --json flag",
		"- second feature",
		"### Bug Fixes",
		"- correct exit code",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "ignored") {
		t.Error("chore commits should not appear in the changelog")
	}
	// Features must come before Bug Fixes.
	if strings.Index(got, "### Features") > strings.Index(got, "### Bug Fixes") {
		t.Error("Features section should precede Bug Fixes")
	}
}

func TestRenderBreaking(t *testing.T) {
	got := Render("2.0.0", "2026-07-26", parse("feat!: drop legacy API"))
	if !strings.Contains(got, "### ⚠ BREAKING CHANGES") {
		t.Errorf("expected breaking-changes header:\n%s", got)
	}
}

func TestRenderEmpty(t *testing.T) {
	got := Render("1.0.1", "2026-07-26", parse("chore: deps"))
	if !strings.Contains(got, "No user-facing changes") {
		t.Errorf("expected empty-release note:\n%s", got)
	}
}

func TestPrependFresh(t *testing.T) {
	out := Prepend("", Render("1.0.0", "2026-07-26", parse("feat: init")))
	if !strings.HasPrefix(out, "# Changelog") {
		t.Errorf("fresh changelog should start with header:\n%s", out)
	}
	if !strings.Contains(out, "## 1.0.0") {
		t.Error("fresh changelog missing release section")
	}
}

func TestPrependExisting(t *testing.T) {
	first := Prepend("", Render("1.0.0", "2026-07-26", parse("feat: init")))
	second := Prepend(first, Render("1.1.0", "2026-07-27", parse("feat: more")))

	i10 := strings.Index(second, "## 1.0.0")
	i11 := strings.Index(second, "## 1.1.0")
	if i11 == -1 || i10 == -1 {
		t.Fatalf("both releases should be present:\n%s", second)
	}
	if i11 > i10 {
		t.Error("newest release should be prepended above older ones")
	}
	if strings.Count(second, "# Changelog") != 1 {
		t.Error("header should appear exactly once")
	}
}
