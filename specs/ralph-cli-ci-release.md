# Plan: CI/Release Setup for ralph-cli

## Files to Create

### 1. `.github/workflows/ci.yml` [DONE]

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - name: Test
        run: go test ./...

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - name: golangci-lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest

  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - name: Build
        run: go build ./cmd/ralph
```

### 2. `.github/workflows/release.yml` [DONE]

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'
  workflow_dispatch:
    inputs:
      tag:
        description: 'Tag to release (e.g., v1.0.0)'
        required: true
        type: string

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Checkout tag (workflow_dispatch)
        if: github.event_name == 'workflow_dispatch'
        run: git checkout ${{ inputs.tag }}

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### 3. `.goreleaser.yaml` [DONE]

```yaml
version: 2

builds:
  - id: ralph
    main: ./cmd/ralph
    binary: ralph
    env:
      - CGO_ENABLED=0
    ldflags:
      - -s -w -X main.version={{.Version}}
    goos:
      - darwin
      - linux
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64

archives:
  - id: ralph
    builds:
      - ralph
    format: tar.gz
    format_overrides:
      - goos: windows
        format: zip
    name_template: "ralph_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: 'checksums.txt'

changelog:
  sort: asc
  filters:
    exclude:
      - '^docs:'
      - '^test:'
```

---

## Testing the Release Pipeline

1. Push the workflow files and goreleaser config
2. Create and push a tag: `git tag v0.1.0 && git push origin v0.1.0`
3. The release workflow will create a GitHub release with:
   - `ralph_0.1.0_darwin_amd64.tar.gz`
   - `ralph_0.1.0_darwin_arm64.tar.gz`
   - `ralph_0.1.0_linux_amd64.tar.gz`
   - `ralph_0.1.0_linux_arm64.tar.gz`
   - `ralph_0.1.0_windows_amd64.zip`
   - `checksums.txt`
