# semver-bump

> Let your commit history decide the next version — and write the changelog for you.

**Status:** 🚧 In development

## Overview

Derive the next semantic version and changelog from Conventional Commits and bump automatically.

## Features

- Parse commits since the last tag as Conventional Commits
- Compute the next semver (major/minor/patch) from commit types
- Generate/update a `CHANGELOG.md` grouped by type
- Create the git tag (dry-run by default)
- GitHub Action wrapper for release automation

## Stack

Go 1.22, `go-git`, Conventional Commits parser, `cobra`.

## Usage

```bash
semver-bump --dry-run
semver-bump --tag
```

## License

MIT
