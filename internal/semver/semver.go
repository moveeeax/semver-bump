// Package semver implements parsing, comparison and bumping of Semantic
// Versions (https://semver.org, spec v2.0.0). It has no external dependencies
// and no side effects, so the precedence rules can be exhaustively unit-tested.
package semver

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Version is a parsed Semantic Version. Prerelease and Build never include
// their leading '-' or '+' separators.
type Version struct {
	Major      uint64
	Minor      uint64
	Patch      uint64
	Prerelease string
	Build      string
}

// ErrInvalid is returned by Parse when the input is not a valid version.
var ErrInvalid = errors.New("invalid semantic version")

// Parse reads a version string, tolerating a single optional leading 'v'.
func Parse(s string) (Version, error) {
	orig := s
	s = strings.TrimPrefix(s, "v")

	var v Version

	// Split off build metadata first: it may itself never contain '+'.
	if i := strings.IndexByte(s, '+'); i >= 0 {
		v.Build = s[i+1:]
		s = s[:i]
		if !isValidDotSeparated(v.Build, true) {
			return Version{}, fmt.Errorf("%w: bad build metadata %q", ErrInvalid, orig)
		}
	}

	// Then the prerelease.
	if i := strings.IndexByte(s, '-'); i >= 0 {
		v.Prerelease = s[i+1:]
		s = s[:i]
		if !isValidPrerelease(v.Prerelease) {
			return Version{}, fmt.Errorf("%w: bad prerelease %q", ErrInvalid, orig)
		}
	}

	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("%w: %q needs MAJOR.MINOR.PATCH", ErrInvalid, orig)
	}
	nums := [3]*uint64{&v.Major, &v.Minor, &v.Patch}
	for i, p := range parts {
		n, err := parseNumericID(p)
		if err != nil {
			return Version{}, fmt.Errorf("%w: %q: %v", ErrInvalid, orig, err)
		}
		*nums[i] = n
	}
	return v, nil
}

// MustParse is Parse but panics on error; convenient in tests and constants.
func MustParse(s string) Version {
	v, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}

// String renders the version in canonical form (without a leading 'v').
func (v Version) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		b.WriteByte('-')
		b.WriteString(v.Prerelease)
	}
	if v.Build != "" {
		b.WriteByte('+')
		b.WriteString(v.Build)
	}
	return b.String()
}

// BumpMajor returns v with the major incremented and lower fields reset.
func (v Version) BumpMajor() Version {
	return Version{Major: v.Major + 1}
}

// BumpMinor returns v with the minor incremented and patch/pre/build reset.
func (v Version) BumpMinor() Version {
	return Version{Major: v.Major, Minor: v.Minor + 1}
}

// BumpPatch returns v with the patch incremented and pre/build reset.
func (v Version) BumpPatch() Version {
	return Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1}
}

// WithPrerelease returns a copy of v with the prerelease set (build cleared).
func (v Version) WithPrerelease(pre string) Version {
	v.Prerelease = pre
	v.Build = ""
	return v
}

// Compare returns -1, 0 or +1 as v precedes, equals or follows other, using
// the precedence rules of the spec. Build metadata is ignored entirely.
func Compare(v, other Version) int {
	if c := cmpUint(v.Major, other.Major); c != 0 {
		return c
	}
	if c := cmpUint(v.Minor, other.Minor); c != 0 {
		return c
	}
	if c := cmpUint(v.Patch, other.Patch); c != 0 {
		return c
	}
	return comparePrerelease(v.Prerelease, other.Prerelease)
}

// Less reports whether v has lower precedence than other.
func (v Version) Less(other Version) bool { return Compare(v, other) < 0 }

// comparePrerelease implements the identifier-by-identifier rules. An empty
// prerelease (a normal release) always outranks a non-empty one.
func comparePrerelease(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return 1 // release > prerelease
	}
	if b == "" {
		return -1
	}
	ap := strings.Split(a, ".")
	bp := strings.Split(b, ".")
	for i := 0; i < len(ap) && i < len(bp); i++ {
		if c := compareIdentifier(ap[i], bp[i]); c != 0 {
			return c
		}
	}
	// All shared identifiers equal: the longer set has higher precedence.
	return cmpInt(len(ap), len(bp))
}

func compareIdentifier(a, b string) int {
	aNum, aIsNum := toNumeric(a)
	bNum, bIsNum := toNumeric(b)
	switch {
	case aIsNum && bIsNum:
		return cmpUint(aNum, bNum)
	case aIsNum: // numeric identifiers always have lower precedence
		return -1
	case bIsNum:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

func toNumeric(s string) (uint64, bool) {
	if s == "" || !isAllDigits(s) {
		return 0, false
	}
	n, err := strconv.ParseUint(s, 10, 64)
	return n, err == nil
}

func parseNumericID(s string) (uint64, error) {
	if s == "" {
		return 0, errors.New("empty numeric identifier")
	}
	if len(s) > 1 && s[0] == '0' {
		return 0, fmt.Errorf("leading zero in %q", s)
	}
	if !isAllDigits(s) {
		return 0, fmt.Errorf("non-numeric %q", s)
	}
	return strconv.ParseUint(s, 10, 64)
}

func isValidPrerelease(s string) bool { return isValidDotSeparated(s, false) }

// isValidDotSeparated validates a dot-separated series of identifiers. When
// build is false the numeric identifiers must not carry leading zeros.
func isValidDotSeparated(s string, build bool) bool {
	if s == "" {
		return false
	}
	for _, id := range strings.Split(s, ".") {
		if id == "" {
			return false
		}
		if !isAlphanumericHyphen(id) {
			return false
		}
		if !build && isAllDigits(id) && len(id) > 1 && id[0] == '0' {
			return false
		}
	}
	return true
}

func isAllDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}

func isAlphanumericHyphen(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '-' {
			continue
		}
		return false
	}
	return true
}

func cmpUint(a, b uint64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
