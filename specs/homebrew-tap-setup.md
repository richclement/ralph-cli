# Plan: Homebrew Tap Setup

## Create the Repository

```bash
gh repo create richclement/homebrew-tap --public --description "Homebrew tap for richclement tools"
cd ~/code
git clone https://github.com/richclement/homebrew-tap.git
cd homebrew-tap
mkdir Formula
```

## Files to Create

### 1. `Formula/ralph-cli.rb`

```ruby
class RalphCli < Formula
  desc "CLI tool for ralph"
  homepage "https://github.com/richclement/ralph-cli"
  version "0.1.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/richclement/ralph-cli/releases/download/v#{version}/ralph_#{version}_darwin_arm64.tar.gz"
      sha256 "PLACEHOLDER"
    else
      url "https://github.com/richclement/ralph-cli/releases/download/v#{version}/ralph_#{version}_darwin_amd64.tar.gz"
      sha256 "PLACEHOLDER"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/richclement/ralph-cli/releases/download/v#{version}/ralph_#{version}_linux_arm64.tar.gz"
      sha256 "PLACEHOLDER"
    else
      url "https://github.com/richclement/ralph-cli/releases/download/v#{version}/ralph_#{version}_linux_amd64.tar.gz"
      sha256 "PLACEHOLDER"
    end
  end

  def install
    bin.install "ralph"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/ralph --version")
  end
end
```

### 2. `README.md`

```markdown
# homebrew-tap

Homebrew formulae for richclement tools.

## Installation

```bash
brew tap richclement/tap
brew install ralph-cli
```

## Available Formulae

- `ralph-cli` - CLI tool for ralph
```

---

## After First Release: Update SHA256 Values

1. Download `checksums.txt` from the GitHub release
2. Update `Formula/ralph-cli.rb` with the real SHA256 values:

```ruby
# darwin_arm64
sha256 "actual_sha256_from_checksums_txt"

# darwin_amd64
sha256 "actual_sha256_from_checksums_txt"

# linux_arm64
sha256 "actual_sha256_from_checksums_txt"

# linux_amd64
sha256 "actual_sha256_from_checksums_txt"
```

3. Commit and push:
```bash
git add Formula/ralph-cli.rb
git commit -m "Update ralph-cli to v0.1.0"
git push
```

---

## Future Release Updates

For each new release:

1. Update `version` line in the formula
2. Update all 4 `sha256` values from the release's `checksums.txt`
3. Commit and push

---

## User Installation

```bash
brew tap richclement/tap
brew install ralph-cli
ralph --version
```
