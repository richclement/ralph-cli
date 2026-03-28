package releaseverify

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type commandResult struct {
	exitCode int
	stdout   string
	stderr   string
}

type cwdMode int

const (
	repoRootMode cwdMode = iota
	freshTempDirMode
)

type commandCase struct {
	name string
	args []string
	cwd  cwdMode
}

func Verify(binaryPath string, version string) error {
	repoRoot, err := resolveRepoRoot()
	if err != nil {
		return fmt.Errorf("resolve repo root: %w", err)
	}

	binaryAbs, err := filepath.Abs(binaryPath)
	if err != nil {
		return fmt.Errorf("resolve binary path: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "ralph-release-verify-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	referenceBinary := filepath.Join(tempDir, "ralph-reference"+filepath.Ext(binaryAbs))
	if err := buildReferenceBinary(repoRoot, referenceBinary, version); err != nil {
		return err
	}

	cases := []commandCase{
		{name: "version", args: []string{"--version"}, cwd: repoRootMode},
		{name: "help", args: []string{"--help"}, cwd: repoRootMode},
		{name: "run-no-prompt", args: []string{"run"}, cwd: freshTempDirMode},
		{name: "run-missing-prompt-file", args: []string{"run", "--prompt-file", "missing.txt"}, cwd: freshTempDirMode},
		{name: "run-no-settings", args: []string{"run", "--prompt", "noop"}, cwd: freshTempDirMode},
	}

	for _, currentCase := range cases {
		expectedCWD, expectedCleanup, err := prepareCWD(tempDir, repoRoot, currentCase)
		if err != nil {
			return fmt.Errorf("prepare expected cwd for %s: %w", currentCase.name, err)
		}
		defer expectedCleanup()

		actualCWD, actualCleanup, err := prepareCWD(tempDir, repoRoot, currentCase)
		if err != nil {
			return fmt.Errorf("prepare actual cwd for %s: %w", currentCase.name, err)
		}
		defer actualCleanup()

		expected, err := runCommand(referenceBinary, currentCase.args, expectedCWD)
		if err != nil {
			return fmt.Errorf("run reference binary for %s: %w", currentCase.name, err)
		}

		actual, err := runCommand(binaryAbs, currentCase.args, actualCWD)
		if err != nil {
			return fmt.Errorf("run release binary for %s: %w", currentCase.name, err)
		}

		if err := compareResults(currentCase.name, expected, actual); err != nil {
			return err
		}
	}

	return nil
}

func resolveRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found in current directory or parents")
		}
		dir = parent
	}
}

func buildReferenceBinary(repoRoot string, outputPath string, version string) error {
	args := []string{"build", "-trimpath", "-o", outputPath}
	if version != "" {
		args = append(args, "-ldflags", "-X main.version="+version)
	}
	args = append(args, "./cmd/ralph")

	cmd := exec.Command("go", args...)
	cmd.Dir = repoRoot

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build reference binary: %w: %s", err, stderr.String())
	}

	return nil
}

func prepareCWD(tempRoot string, repoRoot string, currentCase commandCase) (string, func(), error) {
	switch currentCase.cwd {
	case repoRootMode:
		return repoRoot, func() {}, nil
	case freshTempDirMode:
		dir, err := os.MkdirTemp(tempRoot, sanitizeName(currentCase.name)+"-*")
		if err != nil {
			return "", nil, err
		}
		return dir, func() { _ = os.RemoveAll(dir) }, nil
	default:
		return "", nil, fmt.Errorf("unknown cwd mode %d", currentCase.cwd)
	}
}

func sanitizeName(name string) string {
	return strings.ReplaceAll(name, " ", "-")
}

func runCommand(binary string, args []string, cwd string) (commandResult, error) {
	cmd := exec.Command(binary, args...)
	cmd.Dir = cwd

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := commandResult{
		exitCode: 0,
		stdout:   stdout.String(),
		stderr:   stderr.String(),
	}
	if err == nil {
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.exitCode = exitErr.ExitCode()
		return result, nil
	}

	return commandResult{}, err
}

func compareResults(name string, expected commandResult, actual commandResult) error {
	if expected.exitCode != actual.exitCode {
		return fmt.Errorf("%s exit code mismatch: expected %d, got %d", name, expected.exitCode, actual.exitCode)
	}
	if expected.stdout != actual.stdout {
		return fmt.Errorf("%s stdout mismatch\nexpected:\n%s\nactual:\n%s", name, expected.stdout, actual.stdout)
	}
	if expected.stderr != actual.stderr {
		return fmt.Errorf("%s stderr mismatch\nexpected:\n%s\nactual:\n%s", name, expected.stderr, actual.stderr)
	}

	return nil
}
