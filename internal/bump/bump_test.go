package bump

import (
	"testing"

	"github.com/moveeeax/semver-bump/internal/conventional"
	"github.com/moveeeax/semver-bump/internal/semver"
)

func commits(msgs ...string) []conventional.Commit {
	var out []conventional.Commit
	for _, m := range msgs {
		if c, ok := conventional.Parse(m); ok {
			out = append(out, c)
		}
	}
	return out
}

func TestLevelFor(t *testing.T) {
	cases := []struct {
		name string
		msgs []string
		want Level
	}{
		{"empty", nil, None},
		{"fix only", []string{"fix: a"}, Patch},
		{"feat wins over fix", []string{"fix: a", "feat: b"}, Minor},
		{"breaking wins over feat", []string{"feat: a", "feat!: b"}, Major},
		{"breaking footer", []string{"fix: a\n\nBREAKING CHANGE: x"}, Major},
		{"chore only", []string{"chore: deps"}, None},
		{"perf is patch", []string{"perf: faster"}, Patch},
	}
	for _, c := range cases {
		if got := LevelFor(commits(c.msgs...)); got != c.want {
			t.Errorf("%s: LevelFor = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestNextStable(t *testing.T) {
	cur := semver.MustParse("1.4.2")
	cases := []struct {
		level Level
		want  string
	}{
		{Major, "2.0.0"},
		{Minor, "1.5.0"},
		{Patch, "1.4.3"},
		{None, "1.4.2"},
	}
	for _, c := range cases {
		if got := Next(cur, c.level, false).String(); got != c.want {
			t.Errorf("Next(1.4.2, %v) = %q, want %q", c.level, got, c.want)
		}
	}
}

func TestNextZeroMajorGuard(t *testing.T) {
	cur := semver.MustParse("0.3.1")
	// Breaking is downgraded to minor, feat to patch while in 0.x.
	if got := Next(cur, Major, false).String(); got != "0.4.0" {
		t.Errorf("0.x breaking = %q, want 0.4.0", got)
	}
	if got := Next(cur, Minor, false).String(); got != "0.3.2" {
		t.Errorf("0.x feat = %q, want 0.3.2", got)
	}
	// Opting out gives a real major bump to 1.0.0.
	if got := Next(cur, Major, true).String(); got != "1.0.0" {
		t.Errorf("0.x breaking with allowMajorZero = %q, want 1.0.0", got)
	}
}
