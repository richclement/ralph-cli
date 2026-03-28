---
summary: "Release checklist for ralph-cli (GitHub release + Homebrew tap)"
---

# Releasing `ralph-cli`

Always do **all** steps below (CI + changelog + tag + GitHub release artifacts + tap PR). No partial releases.

Shortcut scripts (preferred, keep notes non-empty):
```sh
scripts/release.sh X.Y.Z
```

Assumptions:
- Repo: `richclement/ralph-cli`
- Tap repo: `richclement/homebrew-tap` (tap: `richclement/tap`)
- Homebrew formula name: `ralph-cli` (installs the `ralph` binary)

## 0) Prereqs
- Clean working tree on `main`.
- Go toolchain installed (Go version comes from `go.mod`).
- `make` works locally.
- GitHub Actions secret `HOMEBREW_TAP_TOKEN` with write access to `richclement/homebrew-tap` contents and pull requests.

## 1) Verify build is green
```sh
make ci
```

Confirm GitHub Actions `ci` is green for the commit you're tagging:
```sh
gh run list -L 5 --branch main
```

## 2) Update changelog
- Update `CHANGELOG.md` for the version you're releasing.

Example heading:
- `## 0.1.0 - 2026-01-06`

## 3) Commit, tag & push
```sh
git checkout main
git pull

# commit changelog + any release tweaks
git commit -am "release: vX.Y.Z"

scripts/release.sh X.Y.Z
```

## 4) Verify GitHub release artifacts
The tag push triggers `.github/workflows/release.yml`. The workflow now:
- runs `quality`
- verifies native binaries on Linux, macOS, and Windows
- cross-builds six archives
- publishes or updates the GitHub Release
- opens or updates a PR against `richclement/homebrew-tap`

```sh
gh run list -L 5 --workflow release.yml
gh release view vX.Y.Z
```

Ensure the release has:
- non-empty notes
- six platform archives
- `checksums.txt`

If the workflow needs a rerun, use GitHub Actions rerun for the existing tag run.

## 5) Review and merge the Homebrew tap PR
The release workflow generates `Formula/ralph-cli.rb` from the release checksums and opens or updates a PR in `richclement/homebrew-tap`.

Verify:
- PR branch is `ralph-cli-release-vX.Y.Z`
- PR updates only `Formula/ralph-cli.rb`
- PR body links back to the GitHub Release and includes the checksum table

Merge the tap PR once it looks correct.

## 6) Optional sanity-check install after the tap PR merges
```sh
brew update
brew uninstall ralph-cli || true
brew untap richclement/tap || true
brew tap richclement/tap
brew install richclement/tap/ralph-cli
brew test richclement/tap/ralph-cli
ralph --version
```

## Notes
- The `ralph --version` command displays the version string.
- Use tags + changelog as the source of truth for release history.
- There is no separate local release verification script anymore. The GitHub Actions release workflow is the source of truth.
