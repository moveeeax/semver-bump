package conventional

import "testing"

func TestParseHeaderVariants(t *testing.T) {
	cases := []struct {
		msg      string
		typ      string
		scope    string
		breaking bool
		desc     string
	}{
		{"feat: add flag", "feat", "", false, "add flag"},
		{"fix(parser): handle empty input", "fix", "parser", false, "handle empty input"},
		{"feat(api)!: drop v1", "feat", "api", true, "drop v1"},
		{"refactor!: rename package", "refactor", "", true, "rename package"},
		{"FEAT: upper type normalised", "feat", "", false, "upper type normalised"},
		{"chore:   trims spaces  ", "chore", "", false, "trims spaces"},
	}
	for _, c := range cases {
		got, ok := Parse(c.msg)
		if !ok {
			t.Fatalf("Parse(%q) failed", c.msg)
		}
		if got.Type != c.typ || got.Scope != c.scope || got.Breaking != c.breaking || got.Description != c.desc {
			t.Errorf("Parse(%q) = %+v, want type=%q scope=%q breaking=%v desc=%q",
				c.msg, got, c.typ, c.scope, c.breaking, c.desc)
		}
	}
}

func TestBreakingFooter(t *testing.T) {
	msg := "feat: new engine\n\nBody text.\n\nBREAKING CHANGE: config format changed"
	c, ok := Parse(msg)
	if !ok || !c.Breaking {
		t.Fatalf("expected breaking commit, got ok=%v breaking=%v", ok, c.Breaking)
	}

	msg2 := "fix: thing\n\nBREAKING-CHANGE: hyphenated footer also counts"
	c2, _ := Parse(msg2)
	if !c2.Breaking {
		t.Error("hyphenated BREAKING-CHANGE footer should mark breaking")
	}
}

func TestNonConventional(t *testing.T) {
	for _, m := range []string{
		"", "just a message", "Merge branch 'main'",
		"feat add flag", // missing colon
		"123: numeric type",
		": empty type",
	} {
		if _, ok := Parse(m); ok {
			t.Errorf("Parse(%q) should not be conventional", m)
		}
	}
}

func TestParseWithHash(t *testing.T) {
	c, ok := ParseWithHash("abc1234", "fix: bug")
	if !ok || c.Hash != "abc1234" {
		t.Errorf("ParseWithHash = %+v ok=%v", c, ok)
	}
}
