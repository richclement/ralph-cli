#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
if [[ -z "$version" ]]; then
  echo "usage: scripts/release.sh X.Y.Z" >&2
  exit 2
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

branch="$(git rev-parse --abbrev-ref HEAD)"
if [[ "$branch" != "main" ]]; then
  echo "expected branch main (got $branch)" >&2
  exit 2
fi
if [[ -n "$(git status --porcelain)" ]]; then
  echo "working tree not clean" >&2
  exit 2
fi

changelog="CHANGELOG.md"
if ! rg -q "^## ${version} - " "$changelog"; then
  echo "missing changelog section for $version" >&2
  exit 2
fi
if rg -q "^## ${version} - Unreleased" "$changelog"; then
  echo "changelog section still Unreleased for $version" >&2
  exit 2
fi

notes_file="$(mktemp -t ralph-release-notes)"
awk -v ver="$version" '
  $0 ~ "^## "ver" " {print "## "ver; in_section=1; next}
  in_section && /^## / {exit}
  in_section {print}
' "$changelog" | sed '/^$/d' > "$notes_file"

if [[ ! -s "$notes_file" ]]; then
  echo "release notes empty for $version" >&2
  exit 2
fi

make ci

if git rev-parse "v$version" >/dev/null 2>&1; then
  echo "tag v$version already exists"
else
  git tag -a "v$version" -m "Release $version"
fi

git push origin main --tags

rm -f "$notes_file"

echo "Tag pushed. Monitor .github/workflows/release.yml for v$version, then review and merge the Homebrew tap PR."
