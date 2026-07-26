// Command semver-bump derives the next semantic version and changelog from
// Conventional Commits and, optionally, creates the git tag.
package main

import (
	"os"

	"github.com/moveeeax/semver-bump/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
