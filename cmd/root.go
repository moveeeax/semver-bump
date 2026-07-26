// Package cmd wires the semver-bump command-line interface.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/moveeeax/semver-bump/internal/changelog"
	"github.com/moveeeax/semver-bump/internal/gitrepo"
	"github.com/moveeeax/semver-bump/internal/release"
)

// version is overridable at build time via -ldflags "-X ...cmd.version=...".
var version = "dev"

type options struct {
	repoPath       string
	changelogPath  string
	prerelease     string
	tag            bool
	allowMajorZero bool
	jsonOut        bool
}

// Execute runs the root command and returns a process exit code.
func Execute() int {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

func newRootCmd() *cobra.Command {
	opt := &options{}
	cmd := &cobra.Command{
		Use:   "semver-bump",
		Short: "Derive the next semantic version and changelog from Conventional Commits",
		Long: "semver-bump reads the commits since the latest semver tag, interprets them as\n" +
			"Conventional Commits, computes the next semantic version and renders a changelog.\n" +
			"By default it only reports what it would do; pass --tag to write the changelog and\n" +
			"create the git tag.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd, opt)
		},
	}

	f := cmd.Flags()
	f.StringVar(&opt.repoPath, "repo", ".", "path to the git repository")
	f.StringVar(&opt.changelogPath, "changelog", "CHANGELOG.md", "changelog file to update with --tag")
	f.StringVar(&opt.prerelease, "prerelease", "", "prerelease channel to append, e.g. rc or beta")
	f.BoolVar(&opt.tag, "tag", false, "write the changelog and create the git tag (otherwise dry-run)")
	f.BoolVar(&opt.allowMajorZero, "allow-major-zero", false, "allow a breaking change in 0.x to bump to 1.0.0")
	f.BoolVar(&opt.jsonOut, "json", false, "print the plan as JSON")
	return cmd
}

func run(cmd *cobra.Command, opt *options) error {
	repo, err := gitrepo.Open(opt.repoPath)
	if err != nil {
		return err
	}
	plan, err := release.Compute(repo, release.Options{
		Prerelease:     opt.prerelease,
		AllowMajorZero: opt.allowMajorZero,
	})
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	if opt.jsonOut {
		return printJSON(out, plan)
	}

	prev := plan.PreviousTag
	if prev == "" {
		prev = "(none)"
	}
	fmt.Fprintf(out, "Previous tag:   %s\n", prev)
	fmt.Fprintf(out, "Commits parsed: %d (%d ignored)\n", len(plan.Commits), plan.Skipped)
	fmt.Fprintf(out, "Bump level:     %s\n", plan.Level)

	if !plan.HasChanges {
		fmt.Fprintf(out, "No release-worthy changes since %s; nothing to do.\n", prev)
		return nil
	}

	fmt.Fprintf(out, "Next version:   %s\n\n", plan.NextVersion)
	fmt.Fprintln(out, plan.ChangelogSection)

	if !opt.tag {
		fmt.Fprintln(out, "Dry run: pass --tag to write the changelog and create the tag.")
		return nil
	}

	if err := writeChangelog(opt.changelogPath, plan.ChangelogSection); err != nil {
		return err
	}
	tagName := "v" + plan.NextVersion.String()
	if err := repo.CreateTag(tagName, "Release "+tagName); err != nil {
		return fmt.Errorf("create tag %s: %w", tagName, err)
	}
	fmt.Fprintf(out, "Updated %s and created tag %s.\n", opt.changelogPath, tagName)
	fmt.Fprintf(out, "Push it with: git push origin %s\n", tagName)
	return nil
}

func writeChangelog(path, section string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	updated := changelog.Prepend(string(existing), section)
	return os.WriteFile(path, []byte(updated), 0o644)
}

func printJSON(w interface{ Write([]byte) (int, error) }, plan release.Plan) error {
	prev := plan.PreviousTag
	payload := map[string]any{
		"previous_tag": prev,
		"current":      plan.CurrentVersion.String(),
		"next":         plan.NextVersion.String(),
		"next_tag":     "v" + plan.NextVersion.String(),
		"level":        plan.Level.String(),
		"has_changes":  plan.HasChanges,
		"commit_count": len(plan.Commits),
		"skipped":      plan.Skipped,
		"changelog":    strings.TrimRight(plan.ChangelogSection, "\n"),
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}
