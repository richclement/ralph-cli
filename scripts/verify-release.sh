#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
if [[ -z "$version" ]]; then
  echo "usage: scripts/verify-release.sh X.Y.Z" >&2
  exit 2
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

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

release_body="$(gh release view "v$version" --json body -q .body)"
if [[ -z "$release_body" ]]; then
  echo "GitHub release notes empty for v$version" >&2
  exit 2
fi

expected_assets=(
  "ralph_${version}_darwin_amd64.tar.gz"
  "ralph_${version}_darwin_arm64.tar.gz"
  "ralph_${version}_linux_amd64.tar.gz"
  "ralph_${version}_linux_arm64.tar.gz"
  "ralph_${version}_windows_amd64.zip"
  "ralph_${version}_windows_arm64.zip"
  "checksums.txt"
)

assets="$(gh release view "v$version" --json assets -q '.assets[].name')"
for asset in "${expected_assets[@]}"; do
  if ! printf '%s\n' "$assets" | rg -x "$asset" >/dev/null; then
    echo "missing release asset for v$version: $asset" >&2
    exit 2
  fi
done

release_run_id="$(gh run list -L 20 --workflow release.yml --json databaseId,conclusion,headBranch,event -q ".[] | select(.headBranch==\"v$version\") | select(.event==\"push\") | select(.conclusion==\"success\") | .databaseId" | head -n1)"
if [[ -z "$release_run_id" ]]; then
  echo "release workflow not green for v$version" >&2
  exit 2
fi

ci_ok="$(gh run list -L 1 --workflow ci --branch main --json conclusion -q '.[0].conclusion')"
if [[ "$ci_ok" != "success" ]]; then
  echo "CI not green for main" >&2
  exit 2
fi

make ci

tap_branch="ralph-cli-release-v$version"
tap_pr_number="$(gh pr list --repo richclement/homebrew-tap --state all --head "richclement:${tap_branch}" --json number -q '.[0].number')"
if [[ -z "$tap_pr_number" ]]; then
  echo "no Homebrew tap PR found for branch $tap_branch" >&2
  exit 2
fi

tap_pr_branch="$(gh pr view "$tap_pr_number" --repo richclement/homebrew-tap --json headRefName -q .headRefName)"
if [[ "$tap_pr_branch" != "$tap_branch" ]]; then
  echo "unexpected tap PR branch: $tap_pr_branch" >&2
  exit 2
fi

tap_pr_file="$(gh pr view "$tap_pr_number" --repo richclement/homebrew-tap --json files -q '.files[].path')"
if ! printf '%s\n' "$tap_pr_file" | rg -x "Formula/ralph-cli.rb" >/dev/null; then
  echo "tap PR does not update Formula/ralph-cli.rb" >&2
  exit 2
fi

rm -f "$notes_file"

echo "Release v$version verified (CI, GitHub release notes/assets, and Homebrew tap PR)."
