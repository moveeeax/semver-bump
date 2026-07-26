package semver

import "testing"

func TestParseValid(t *testing.T) {
	cases := []struct {
		in                  string
		major, minor, patch uint64
		pre, build          string
	}{
		{"0.0.0", 0, 0, 0, "", ""},
		{"1.2.3", 1, 2, 3, "", ""},
		{"v1.2.3", 1, 2, 3, "", ""},
		{"10.20.30", 10, 20, 30, "", ""},
		{"1.0.0-alpha", 1, 0, 0, "alpha", ""},
		{"1.0.0-alpha.1", 1, 0, 0, "alpha.1", ""},
		{"1.0.0-0.3.7", 1, 0, 0, "0.3.7", ""},
		{"1.0.0-x.7.z.92", 1, 0, 0, "x.7.z.92", ""},
		{"1.0.0+20130313144700", 1, 0, 0, "", "20130313144700"},
		{"1.0.0-beta+exp.sha.5114f85", 1, 0, 0, "beta", "exp.sha.5114f85"},
		{"1.0.0+21AF26D3---117B344092BD", 1, 0, 0, "", "21AF26D3---117B344092BD"},
		{"1.0.0-rc.1+build.1", 1, 0, 0, "rc.1", "build.1"},
	}
	for _, c := range cases {
		v, err := Parse(c.in)
		if err != nil {
			t.Fatalf("Parse(%q) unexpected error: %v", c.in, err)
		}
		if v.Major != c.major || v.Minor != c.minor || v.Patch != c.patch || v.Prerelease != c.pre || v.Build != c.build {
			t.Errorf("Parse(%q) = %+v, want %d.%d.%d pre=%q build=%q", c.in, v, c.major, c.minor, c.patch, c.pre, c.build)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	bad := []string{
		"", "1", "1.2", "1.2.3.4", "a.b.c", "1.2.x",
		"01.2.3", "1.02.3", "1.2.03", // leading zeros in core
		"1.2.3-", "1.2.3+", "1.2.3-01", // leading zero numeric prerelease id
		"1.2.3-beta_1", "1.2.3-+build", "-1.2.3",
		"1.2.3-alpha..1", // empty identifier
	}
	for _, s := range bad {
		if v, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) = %+v, want error", s, v)
		}
	}
}

func TestStringRoundTrip(t *testing.T) {
	for _, s := range []string{"1.2.3", "1.0.0-alpha.1", "1.0.0-beta+exp.sha.5114f85", "0.0.0+build"} {
		v := MustParse(s)
		if got := v.String(); got != s {
			t.Errorf("String() = %q, want %q", got, s)
		}
	}
}

func TestBump(t *testing.T) {
	v := MustParse("1.2.3-rc.1+meta")
	if got := v.BumpMajor().String(); got != "2.0.0" {
		t.Errorf("BumpMajor = %q, want 2.0.0", got)
	}
	if got := v.BumpMinor().String(); got != "1.3.0" {
		t.Errorf("BumpMinor = %q, want 1.3.0", got)
	}
	if got := v.BumpPatch().String(); got != "1.2.4" {
		t.Errorf("BumpPatch = %q, want 1.2.4", got)
	}
}

// TestPrecedence exercises the ordering example from the spec, section 11.
func TestPrecedence(t *testing.T) {
	ordered := []string{
		"1.0.0-alpha",
		"1.0.0-alpha.1",
		"1.0.0-alpha.beta",
		"1.0.0-beta",
		"1.0.0-beta.2",
		"1.0.0-beta.11",
		"1.0.0-rc.1",
		"1.0.0",
	}
	for i := 0; i < len(ordered); i++ {
		for j := 0; j < len(ordered); j++ {
			a := MustParse(ordered[i])
			b := MustParse(ordered[j])
			want := cmpInt(i, j)
			if got := Compare(a, b); got != want {
				t.Errorf("Compare(%q,%q) = %d, want %d", ordered[i], ordered[j], got, want)
			}
		}
	}
}

func TestPrereleaseIsLowerThanRelease(t *testing.T) {
	if !MustParse("1.0.0-alpha").Less(MustParse("1.0.0")) {
		t.Error("1.0.0-alpha should be < 1.0.0")
	}
	if MustParse("1.0.0").Less(MustParse("1.0.0-alpha")) {
		t.Error("1.0.0 should not be < 1.0.0-alpha")
	}
}

func TestBuildMetadataIgnored(t *testing.T) {
	a := MustParse("1.0.0+build.1")
	b := MustParse("1.0.0+build.999")
	if Compare(a, b) != 0 {
		t.Error("build metadata must not affect precedence")
	}
	if Compare(MustParse("1.0.0-alpha+a"), MustParse("1.0.0-alpha+b")) != 0 {
		t.Error("build metadata must not affect precedence with prerelease")
	}
}

func TestNumericVsAlphaIdentifier(t *testing.T) {
	// Numeric identifiers always have lower precedence than alphanumeric ones.
	if Compare(MustParse("1.0.0-1"), MustParse("1.0.0-alpha")) != -1 {
		t.Error("numeric identifier should rank below alphanumeric")
	}
	// Larger numeric identifiers win.
	if Compare(MustParse("1.0.0-2"), MustParse("1.0.0-11")) != -1 {
		t.Error("numeric identifiers compare numerically, not lexically")
	}
}

func TestCorePrecedence(t *testing.T) {
	if Compare(MustParse("2.1.1"), MustParse("2.1.0")) != 1 {
		t.Error("2.1.1 > 2.1.0")
	}
	if Compare(MustParse("2.0.0"), MustParse("2.1.0")) != -1 {
		t.Error("2.0.0 < 2.1.0")
	}
	if Compare(MustParse("1.0.0"), MustParse("1.0.0")) != 0 {
		t.Error("1.0.0 == 1.0.0")
	}
}
