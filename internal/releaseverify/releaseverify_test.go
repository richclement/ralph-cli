package releaseverify

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyMatchesReferenceBinary(t *testing.T) {
	t.Parallel()

	repoRoot := mustRepoRoot(t)
	binaryPath := filepath.Join(t.TempDir(), "ralph"+exeSuffix())

	buildBinary(t, repoRoot, binaryPath, "1.2.3")

	if err := Verify(binaryPath, "1.2.3"); err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
}

func TestVerifyDetectsMismatch(t *testing.T) {
	t.Parallel()

	repoRoot := mustRepoRoot(t)
	binaryPath := filepath.Join(t.TempDir(), "ralph"+exeSuffix())

	buildBinary(t, repoRoot, binaryPath, "9.9.9")

	err := Verify(binaryPath, "1.2.3")
	if err == nil {
		t.Fatal("expected mismatch error")
	}
	if !strings.Contains(err.Error(), "version stdout mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyMissingBinary(t *testing.T) {
	t.Parallel()

	err := Verify(filepath.Join(t.TempDir(), "missing"+exeSuffix()), "1.2.3")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func mustRepoRoot(t *testing.T) string {
	t.Helper()

	root, err := resolveRepoRoot()
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

func buildBinary(t *testing.T, repoRoot string, outputPath string, version string) {
	t.Helper()

	cmd := exec.Command("go", "build", "-trimpath", "-ldflags", "-X main.version="+version, "-o", outputPath, "./cmd/ralph")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build binary: %v\n%s", err, output)
	}
}

func exeSuffix() string {
	if filepath.Separator == '\\' {
		return ".exe"
	}
	return ""
}
