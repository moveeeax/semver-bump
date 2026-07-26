# semver-bump

[![CI](https://github.com/moveeeax/semver-bump/actions/workflows/ci.yml/badge.svg)](https://github.com/moveeeax/semver-bump/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

> Let your commit history decide the next version — and write the changelog for you.

`semver-bump` reads the commits since your latest semver tag, interprets them as
[Conventional Commits](https://www.conventionalcommits.org), computes the next
[Semantic Version](https://semver.org) and renders a changelog grouped by type.
By default it only tells you what it would do; pass `--tag` to write the
changelog and create the git tag.

No release manifests, no hand-edited version constants, no arguing about whether
that fix was a patch or a minor. The commits already carry the answer.

## What it does

- **Parses** every commit since the last `vMAJOR.MINOR.PATCH` tag as a
  Conventional Commit (`type(scope)!: description`, plus `BREAKING CHANGE`
  footers). Non-conforming commits are counted and ignored.
- **Decides** the bump: any breaking change → **major**, otherwise any `feat` →
  **minor**, otherwise any `fix`/`perf`/`revert`/`refactor` → **patch**. Nothing
  release-worthy → no bump.
- **Renders** a `CHANGELOG.md` section grouped into Breaking Changes, Features,
  Bug Fixes and friends, sorted by scope.
- **Tags** the new version as an annotated git tag — but only with `--tag`; the
  default is a safe dry-run.
- **Automates** releases through a composite GitHub Action (`action.yml`).

## How it works

The interesting logic is pure and has no idea git exists. A `semver` package
implements the full precedence spec — including the parts people get wrong:
`1.0.0-alpha < 1.0.0`, numeric identifiers rank below alphanumeric ones
(`1.0.0-1 < 1.0.0-alpha`), prerelease identifiers compare numerically not
lexically (`beta.2 < beta.11`), and build metadata is ignored entirely. All git
access sits behind a small `Repo` interface, so the version maths and changelog
rendering are tested against an in-memory fake with zero I/O.

One notable detail: while a project is still in `0.x`, a breaking change does
**not** promote it to `1.0.0`. `semver-bump` downgrades a major-level change to a
minor bump (and minor to patch) so it never declares your unstable project stable
on your behalf. Pass `--allow-major-zero` when you actually mean it.

## Install

```bash
go install github.com/moveeeax/semver-bump@latest
```

Or build from source:

```bash
git clone https://github.com/moveeeax/semver-bump
cd semver-bump && make build   # produces bin/semver-bump
```

## Usage

Dry-run (the default) prints the plan and the changelog section it would write:

```console
$ semver-bump
Previous tag:   v1.0.0
Commits parsed: 3 (0 ignored)
Bump level:     major
Next version:   2.0.0

## 2.0.0 - 2026-07-26

### ⚠ BREAKING CHANGES

- **cli:** add --json flag (7cc13d1)

### Features

- **cli:** add --json flag (7cc13d1)

### Bug Fixes

- correct a bug (e29866c)

Dry run: pass --tag to write the changelog and create the tag.
```

Commit the release for real:

```bash
semver-bump --tag           # writes CHANGELOG.md and creates the annotated tag
git push origin v2.0.0
```

Machine-readable output for scripts and CI:

```bash
semver-bump --json | jq -r .next_tag
```

### Flags

| Flag | Default | Description |
| --- | --- | --- |
| `--repo` | `.` | Path to the git repository |
| `--tag` | `false` | Write the changelog and create the tag (otherwise dry-run) |
| `--prerelease` | | Prerelease channel to append, e.g. `rc` → `2.1.0-rc.1` |
| `--changelog` | `CHANGELOG.md` | Changelog file updated with `--tag` |
| `--allow-major-zero` | `false` | Allow a breaking change in `0.x` to bump to `1.0.0` |
| `--json` | `false` | Print the plan as JSON |

## GitHub Action

Use the bundled composite action to cut releases from your pipeline:

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0            # full history + tags are required
- id: bump
  uses: moveeeax/semver-bump@v1
  with:
    tag: "true"
- if: steps.bump.outputs.has-changes == 'true'
  run: git push origin ${{ steps.bump.outputs.next-tag }}
```

Outputs: `next`, `next-tag`, `level`, `has-changes`. A full example lives in
[`examples/release.yml`](examples/release.yml).

## Development

```bash
make all     # gofmt check, go vet, go test -race, build
make smoke   # end-to-end smoke test against a throwaway repo
```

## License

[MIT](LICENSE)
