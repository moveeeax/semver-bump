#!/usr/bin/env bash
# Build semver-bump, run it against a throwaway repository and assert that the
# computed versions match the Conventional Commit rules. Used by CI and handy
# to run locally: `bash examples/smoke_test.sh`.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
bindir="$(mktemp -d)"
bin="$bindir/semver-bump"

echo "==> building"
go build -C "$root" -o "$bin" .

work="$(mktemp -d)"
cd "$work"
git init -q
git config user.email smoke@test.local
git config user.name smoke

echo "==> case 1: fix since v1.0.0 => patch"
echo a >a && git add -A && git commit -qm "feat: first feature"
git tag v1.0.0
echo b >b && git add -A && git commit -qm "fix: correct a bug"
out="$("$bin" --repo "$work" --json)"
echo "$out"
next="$(echo "$out" | grep -o '"next": "[^"]*"' | cut -d'"' -f4)"
[ "$next" = "1.0.1" ] || { echo "FAIL: expected 1.0.1, got '$next'"; exit 1; }

echo "==> case 2: breaking change => major"
echo c >c && git add -A && git commit -qm "feat!: incompatible change"
out="$("$bin" --repo "$work" --json)"
next="$(echo "$out" | grep -o '"next": "[^"]*"' | cut -d'"' -f4)"
[ "$next" = "2.0.0" ] || { echo "FAIL: expected 2.0.0, got '$next'"; exit 1; }

echo "==> case 3: --tag writes changelog and creates the tag"
"$bin" --repo "$work" --tag >/dev/null
git -C "$work" rev-parse v2.0.0 >/dev/null || { echo "FAIL: tag v2.0.0 not created"; exit 1; }
grep -q "## 2.0.0" "$work/CHANGELOG.md" || { echo "FAIL: changelog not written"; exit 1; }

echo "smoke test passed"
