// Package conventional parses Git commit messages that follow the
// Conventional Commits 1.0.0 specification (https://www.conventionalcommits.org).
// It is pure string handling: no git, no I/O.
package conventional

import (
	"regexp"
	"strings"
)

// Commit is a parsed Conventional Commit.
type Commit struct {
	Type        string // "feat", "fix", "chore", ...
	Scope       string // optional, without parentheses
	Breaking    bool   // "!" marker or a "BREAKING CHANGE" footer
	Description string // the summary text after the colon
	Body        string // everything after the header, trimmed
	Hash        string // short commit hash, when known

	// Set only when the entry comes from a pull request rather than a commit.
	PR                int    // pull request number
	Author            string // GitHub login to credit; empty for bots
	FirstContribution bool   // the author's first merged pull request in the repo
}

// headerRe matches "type(scope)!: description". Scope and the "!" are optional.
var headerRe = regexp.MustCompile(`^([a-zA-Z]+)(?:\(([^)]*)\))?(!)?:[ \t]+(.+)$`)

// breakingFooterRe matches a "BREAKING CHANGE:" or "BREAKING-CHANGE:" footer.
var breakingFooterRe = regexp.MustCompile(`(?m)^BREAKING[ -]CHANGE:[ \t]*(.+)$`)

// Parse turns a raw commit message into a Commit. The second return value is
// false when the header does not conform to the Conventional Commits grammar.
func Parse(message string) (Commit, bool) {
	message = strings.ReplaceAll(message, "\r\n", "\n")
	lines := strings.SplitN(message, "\n", 2)
	header := strings.TrimSpace(lines[0])

	m := headerRe.FindStringSubmatch(header)
	if m == nil {
		return Commit{}, false
	}

	c := Commit{
		Type:        strings.ToLower(m[1]),
		Scope:       m[2],
		Breaking:    m[3] == "!",
		Description: strings.TrimSpace(m[4]),
	}
	if len(lines) > 1 {
		c.Body = strings.TrimSpace(lines[1])
		if breakingFooterRe.MatchString(c.Body) {
			c.Breaking = true
		}
	}
	return c, true
}

// ParseWithHash is Parse with the commit hash attached to the result.
func ParseWithHash(hash, message string) (Commit, bool) {
	c, ok := Parse(message)
	if ok {
		c.Hash = hash
	}
	return c, ok
}
