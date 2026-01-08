package agent

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/richclement/ralph-cli/internal/config"
)

func TestShellQuote(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple string",
			input: "claude",
			want:  "claude",
		},
		{
			name:  "string with spaces",
			input: "my command",
			want:  "'my command'",
		},
		{
			name:  "string with single quote",
			input: "it's",
			want:  "'it'\"'\"'s'",
		},
		{
			name:  "string with special chars",
			input: "test$var",
			want:  "'test$var'",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "path with spaces",
			input: "/path/to/my agent",
			want:  "'/path/to/my agent'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellQuote(tt.input)
			if got != tt.want {
				t.Errorf("shellQuote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildArgs_Claude(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "claude",
			Flags:   []string{"--model", "opus"},
		},
	}
	r := NewRunner(settings)

	args, promptFile := r.buildArgs("test prompt", 1, RunOptions{})

	// Should have -p flag for claude
	if len(args) < 1 || args[0] != "-p" {
		t.Errorf("Expected first arg to be -p, got %v", args)
	}

	// Should include user flags
	if len(args) < 3 || args[1] != "--model" || args[2] != "opus" {
		t.Errorf("Expected user flags, got %v", args)
	}

	// Should include prompt
	if len(args) < 4 || args[3] != "test prompt" {
		t.Errorf("Expected prompt as last arg, got %v", args)
	}

	// No prompt file for claude
	if promptFile != "" {
		t.Errorf("Expected no prompt file for claude, got %q", promptFile)
	}
}

func TestBuildArgs_Amp(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "amp",
		},
	}
	r := NewRunner(settings)

	args, _ := r.buildArgs("test prompt", 1, RunOptions{})

	// For amp, -x should immediately precede the prompt (at the end)
	if len(args) < 2 {
		t.Fatalf("Expected at least 2 args, got %v", args)
	}

	// Last two args should be -x and prompt
	if args[len(args)-2] != "-x" {
		t.Errorf("Expected -x as second-to-last arg, got %v", args)
	}
	if args[len(args)-1] != "test prompt" {
		t.Errorf("Expected prompt as last arg, got %v", args)
	}
}

func TestBuildArgs_UnknownAgent(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "custom-agent",
			Flags:   []string{"--custom-flag"},
		},
	}
	r := NewRunner(settings)

	args, _ := r.buildArgs("test prompt", 1, RunOptions{})

	// Should NOT have any inferred flag
	if len(args) < 1 || args[0] == "-p" || args[0] == "-x" || args[0] == "e" {
		t.Errorf("Should not have inferred flag for unknown agent, got %v", args)
	}

	// Should include user flags
	if args[0] != "--custom-flag" {
		t.Errorf("Expected user flag first, got %v", args)
	}

	// Should include prompt
	if args[1] != "test prompt" {
		t.Errorf("Expected prompt, got %v", args)
	}
}

func TestBuildArgs_PathWithExtension(t *testing.T) {
	// Test that Windows .exe extension is handled
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "claude.exe",
		},
	}
	r := NewRunner(settings)

	args, _ := r.buildArgs("test", 1, RunOptions{})

	// Should still detect claude and add -p
	if len(args) < 1 || args[0] != "-p" {
		t.Errorf("Expected -p flag for claude.exe, got %v", args)
	}
}

func TestBuildArgs_FullPath(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "/usr/local/bin/claude",
		},
	}
	r := NewRunner(settings)

	args, _ := r.buildArgs("test", 1, RunOptions{})

	// Should extract basename and detect claude
	if len(args) < 1 || args[0] != "-p" {
		t.Errorf("Expected -p flag for /usr/local/bin/claude, got %v", args)
	}
}

func TestNewRunner(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "claude",
			Flags:   []string{"--model", "opus"},
		},
		StreamAgentOutput: true,
	}

	runner := NewRunner(settings)

	if runner.Settings != settings {
		t.Error("Expected settings to be assigned")
	}
	if runner.Stdout != os.Stdout {
		t.Error("Expected Stdout to default to os.Stdout")
	}
	if runner.Stderr != os.Stderr {
		t.Error("Expected Stderr to default to os.Stderr")
	}
	if runner.Verbose != false {
		t.Error("Expected Verbose to default to false")
	}
}

func TestRunShell_Success(t *testing.T) {
	var stdout, stderr bytes.Buffer

	output, err := RunShell(context.Background(), "echo hello", false, &stdout, &stderr)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if output != "hello\n" {
		t.Errorf("Expected 'hello\\n', got %q", output)
	}
}

func TestRunShell_WithStream(t *testing.T) {
	var stdout, stderr bytes.Buffer

	output, err := RunShell(context.Background(), "echo hello", true, &stdout, &stderr)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if output != "hello\n" {
		t.Errorf("Expected 'hello\\n', got %q", output)
	}
	// When streaming, output should also go to stdout
	if stdout.String() != "hello\n" {
		t.Errorf("Expected stdout to contain 'hello\\n', got %q", stdout.String())
	}
}

func TestRunShell_CommandFailure(t *testing.T) {
	var stdout, stderr bytes.Buffer

	_, err := RunShell(context.Background(), "exit 1", false, &stdout, &stderr)

	if err == nil {
		t.Error("Expected error for failed command")
	}
}

func TestRunShell_CancelledContext(t *testing.T) {
	var stdout, stderr bytes.Buffer

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := RunShell(ctx, "sleep 10", false, &stdout, &stderr)

	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}

func TestBuildArgs_Codex(t *testing.T) {
	// Create .ralph directory for prompt file
	if err := os.MkdirAll(RalphDir, 0o755); err != nil {
		t.Fatalf("Failed to create .ralph directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(RalphDir) }()

	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "codex",
		},
		StreamAgentOutput: false, // No streaming flags
	}
	r := NewRunner(settings)

	args, promptFile := r.buildArgs("test prompt", 1, RunOptions{})

	// Should have "e" subcommand for codex
	if len(args) < 1 || args[0] != "e" {
		t.Errorf("Expected first arg to be 'e', got %v", args)
	}

	// Should have prompt file
	if promptFile == "" {
		t.Error("Expected prompt file for codex")
	}

	// Prompt file should be last arg
	if len(args) < 2 || args[len(args)-1] != promptFile {
		t.Errorf("Expected prompt file as last arg, got %v", args)
	}

	// Should NOT have --json or --full-auto when streaming is disabled
	for _, arg := range args {
		if arg == "--json" || arg == "--full-auto" {
			t.Errorf("Should not have streaming flags when StreamAgentOutput is false, got %v", args)
		}
	}
}

func TestWriteAndFlush_BasicWrite(t *testing.T) {
	var buf bytes.Buffer
	data := []byte("test data")

	err := writeAndFlush([]io.Writer{&buf}, data)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if buf.String() != "test data" {
		t.Errorf("Expected 'test data', got %q", buf.String())
	}
}

func TestWriteAndFlush_MultipleWriters(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	data := []byte("multi writer test")

	err := writeAndFlush([]io.Writer{&buf1, &buf2}, data)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if buf1.String() != "multi writer test" {
		t.Errorf("Expected 'multi writer test' in buf1, got %q", buf1.String())
	}
	if buf2.String() != "multi writer test" {
		t.Errorf("Expected 'multi writer test' in buf2, got %q", buf2.String())
	}
}

// mockFlushWriter is a writer that implements flushWriter
type mockFlushWriter struct {
	buf     bytes.Buffer
	flushed bool
}

func (m *mockFlushWriter) Write(p []byte) (n int, err error) {
	return m.buf.Write(p)
}

func (m *mockFlushWriter) Flush() error {
	m.flushed = true
	return nil
}

func TestWriteAndFlush_WithFlushWriter(t *testing.T) {
	fw := &mockFlushWriter{}
	data := []byte("flush test")

	err := writeAndFlush([]io.Writer{fw}, data)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if fw.buf.String() != "flush test" {
		t.Errorf("Expected 'flush test', got %q", fw.buf.String())
	}
	if !fw.flushed {
		t.Error("Expected Flush to be called")
	}
}

// mockFlushWriterNoErr is a writer that implements flushWriterNoErr
type mockFlushWriterNoErr struct {
	buf     bytes.Buffer
	flushed bool
}

func (m *mockFlushWriterNoErr) Write(p []byte) (n int, err error) {
	return m.buf.Write(p)
}

func (m *mockFlushWriterNoErr) Flush() {
	m.flushed = true
}

func TestWriteAndFlush_WithFlushWriterNoErr(t *testing.T) {
	fw := &mockFlushWriterNoErr{}
	data := []byte("flush no err test")

	err := writeAndFlush([]io.Writer{fw}, data)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if fw.buf.String() != "flush no err test" {
		t.Errorf("Expected 'flush no err test', got %q", fw.buf.String())
	}
	if !fw.flushed {
		t.Error("Expected Flush to be called")
	}
}

// errorWriter always returns an error
type errorWriter struct{}

func (e *errorWriter) Write(p []byte) (n int, err error) {
	return 0, io.ErrClosedPipe
}

func TestWriteAndFlush_WriteError(t *testing.T) {
	ew := &errorWriter{}
	data := []byte("error test")

	err := writeAndFlush([]io.Writer{ew}, data)
	if err == nil {
		t.Error("Expected error, got nil")
	}
	if err != io.ErrClosedPipe {
		t.Errorf("Expected ErrClosedPipe, got %v", err)
	}
}

// shortWriter writes less than requested
type shortWriter struct{}

func (s *shortWriter) Write(p []byte) (n int, err error) {
	if len(p) > 0 {
		return len(p) - 1, nil // Write one less than requested
	}
	return 0, nil
}

func TestWriteAndFlush_ShortWrite(t *testing.T) {
	sw := &shortWriter{}
	data := []byte("short write test")

	err := writeAndFlush([]io.Writer{sw}, data)
	if err == nil {
		t.Error("Expected error for short write")
	}
	if err != io.ErrShortWrite {
		t.Errorf("Expected ErrShortWrite, got %v", err)
	}
}

func TestStreamPTY_BasicRead(t *testing.T) {
	// Create a pipe to simulate PTY
	r, w := io.Pipe()

	var buf bytes.Buffer
	done := make(chan error, 1)

	go func() {
		done <- streamPTY(r, &buf)
	}()

	// Write some data and close
	_, _ = w.Write([]byte("test output"))
	_ = w.Close()

	err := <-done
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if buf.String() != "test output" {
		t.Errorf("Expected 'test output', got %q", buf.String())
	}
}

func TestStreamPTY_MultipleWrites(t *testing.T) {
	r, w := io.Pipe()

	var buf bytes.Buffer
	done := make(chan error, 1)

	go func() {
		done <- streamPTY(r, &buf)
	}()

	// Write multiple chunks
	_, _ = w.Write([]byte("chunk1"))
	_, _ = w.Write([]byte("chunk2"))
	_, _ = w.Write([]byte("chunk3"))
	_ = w.Close()

	err := <-done
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if buf.String() != "chunk1chunk2chunk3" {
		t.Errorf("Expected 'chunk1chunk2chunk3', got %q", buf.String())
	}
}

func TestStreamPTY_MultipleWriters(t *testing.T) {
	r, w := io.Pipe()

	var buf1, buf2 bytes.Buffer
	done := make(chan error, 1)

	go func() {
		done <- streamPTY(r, &buf1, &buf2)
	}()

	_, _ = w.Write([]byte("multi"))
	_ = w.Close()

	err := <-done
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if buf1.String() != "multi" {
		t.Errorf("Expected 'multi' in buf1, got %q", buf1.String())
	}
	if buf2.String() != "multi" {
		t.Errorf("Expected 'multi' in buf2, got %q", buf2.String())
	}
}

func TestStreamPTY_WriteError(t *testing.T) {
	r, w := io.Pipe()

	ew := &errorWriter{}
	done := make(chan error, 1)

	go func() {
		done <- streamPTY(r, ew)
	}()

	_, _ = w.Write([]byte("test"))
	_ = w.Close()

	err := <-done
	if err == nil {
		t.Error("Expected error from streamPTY")
	}
}

func TestRunner_Run_BasicExecution(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "echo",
		},
		StreamAgentOutput: false,
	}

	var stdout, stderr bytes.Buffer
	runner := NewRunner(settings)
	runner.Stdout = &stdout
	runner.Stderr = &stderr

	output, err := runner.Run(context.Background(), "hello", 1)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	// Output should contain "hello" (echo command)
	if output == "" {
		t.Error("Expected non-empty output")
	}
}

func TestRunner_Run_WithStreaming(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "echo",
		},
		StreamAgentOutput: true,
	}

	var stdout, stderr bytes.Buffer
	runner := NewRunner(settings)
	runner.Stdout = &stdout
	runner.Stderr = &stderr

	output, err := runner.Run(context.Background(), "streamed", 1)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if output == "" {
		t.Error("Expected non-empty output")
	}
}

func TestRunner_Run_CancelledContext(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "sleep",
		},
		StreamAgentOutput: false,
	}

	var stdout, stderr bytes.Buffer
	runner := NewRunner(settings)
	runner.Stdout = &stdout
	runner.Stderr = &stderr

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := runner.Run(ctx, "10", 1)
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}

func TestRunner_Run_WithPromptFile(t *testing.T) {
	// Create .ralph directory
	if err := os.MkdirAll(RalphDir, 0o755); err != nil {
		t.Fatalf("Failed to create .ralph directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(RalphDir) }()

	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "codex",
		},
		StreamAgentOutput: false,
	}

	var stdout, stderr bytes.Buffer
	runner := NewRunner(settings)
	runner.Stdout = &stdout
	runner.Stderr = &stderr

	// The run will fail since codex isn't installed, but the prompt file should be created and cleaned up
	_, _ = runner.Run(context.Background(), "test prompt content", 1)

	// Verify prompt file was cleaned up
	promptFile := ".ralph/prompt_001.txt"
	if _, err := os.Stat(promptFile); !os.IsNotExist(err) {
		t.Error("Expected prompt file to be cleaned up")
	}
}

func TestRunner_Run_VerboseLogging(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "echo",
		},
		StreamAgentOutput: true,
	}

	var stdout, stderr bytes.Buffer
	runner := NewRunner(settings)
	runner.Stdout = &stdout
	runner.Stderr = &stderr
	runner.Verbose = true

	_, _ = runner.Run(context.Background(), "verbose test", 1)

	// Stderr should contain verbose output (the [ralph] prefix for agent command is always printed)
	if !bytes.Contains(stderr.Bytes(), []byte("[ralph]")) {
		t.Error("Expected verbose output in stderr")
	}
}

func TestRunner_RunTextMode_BasicExecution(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "echo",
		},
		StreamAgentOutput: true, // Should be ignored in text mode
	}

	var stdout, stderr bytes.Buffer
	runner := NewRunner(settings)
	runner.Stdout = &stdout
	runner.Stderr = &stderr

	output, err := runner.RunTextMode(context.Background(), "hello", 1)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	// Output should contain "hello" (echo command)
	if output == "" {
		t.Error("Expected non-empty output")
	}
}

func TestRunner_RunTextMode_CancelledContext(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "sleep",
		},
	}

	var stdout, stderr bytes.Buffer
	runner := NewRunner(settings)
	runner.Stdout = &stdout
	runner.Stderr = &stderr

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := runner.RunTextMode(ctx, "10", 1)
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}

func TestBuildArgs_ClaudeTextMode(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "claude",
			Flags:   []string{"--model", "opus"},
		},
		StreamAgentOutput: true, // Would normally add stream flags
	}
	r := NewRunner(settings)

	args, promptFile := r.buildArgs("test prompt", 1, RunOptions{TextMode: true})

	// Should have -p flag for claude
	if len(args) < 1 || args[0] != "-p" {
		t.Errorf("Expected first arg to be -p, got %v", args)
	}

	// Should have --output-format text for text mode
	foundOutputFormat := false
	for i, arg := range args {
		if arg == "--output-format" && i+1 < len(args) && args[i+1] == "text" {
			foundOutputFormat = true
			break
		}
	}
	if !foundOutputFormat {
		t.Errorf("Expected --output-format text in args, got %v", args)
	}

	// Should NOT have stream output flags
	for _, arg := range args {
		if arg == "--output-format" {
			// Check that it's followed by "text", not "stream-json"
			continue
		}
		if arg == "stream-json" {
			t.Errorf("Should not have stream-json in text mode, got %v", args)
		}
	}

	// Should include user flags
	foundModel := false
	for i, arg := range args {
		if arg == "--model" && i+1 < len(args) && args[i+1] == "opus" {
			foundModel = true
			break
		}
	}
	if !foundModel {
		t.Errorf("Expected user flags, got %v", args)
	}

	// No prompt file for claude
	if promptFile != "" {
		t.Errorf("Expected no prompt file for claude, got %q", promptFile)
	}
}

func TestBuildArgs_AmpTextMode(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "amp",
			Flags:   []string{"--some-flag"},
		},
		StreamAgentOutput: true, // Would normally add stream flags
	}
	r := NewRunner(settings)

	args, _ := r.buildArgs("test prompt", 1, RunOptions{TextMode: true})

	// For amp, -x should be second-to-last, prompt last
	if len(args) < 2 {
		t.Fatalf("Expected at least 2 args, got %v", args)
	}
	if args[len(args)-2] != "-x" {
		t.Errorf("Expected -x as second-to-last arg, got %v", args)
	}
	if args[len(args)-1] != "test prompt" {
		t.Errorf("Expected prompt as last arg, got %v", args)
	}

	// Should have --dangerously-allow-all for text mode
	foundAutonomy := false
	for _, arg := range args {
		if arg == "--dangerously-allow-all" {
			foundAutonomy = true
			break
		}
	}
	if !foundAutonomy {
		t.Errorf("Expected --dangerously-allow-all in args for text mode, got %v", args)
	}

	// Should NOT have --stream-json in text mode
	for _, arg := range args {
		if arg == "--stream-json" {
			t.Errorf("Should not have --stream-json in text mode, got %v", args)
		}
	}
}

func TestBuildArgs_AmpStreamMode(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "amp",
		},
		StreamAgentOutput: true,
	}
	r := NewRunner(settings)

	args, _ := r.buildArgs("test prompt", 1, RunOptions{})

	// For amp, -x should be second-to-last, prompt last
	if len(args) < 2 {
		t.Fatalf("Expected at least 2 args, got %v", args)
	}
	if args[len(args)-2] != "-x" {
		t.Errorf("Expected -x as second-to-last arg, got %v", args)
	}
	if args[len(args)-1] != "test prompt" {
		t.Errorf("Expected prompt as last arg, got %v", args)
	}

	// Should have --stream-json for streaming mode
	foundStreamJSON := false
	for _, arg := range args {
		if arg == "--stream-json" {
			foundStreamJSON = true
			break
		}
	}
	if !foundStreamJSON {
		t.Errorf("Expected --stream-json in args for streaming mode, got %v", args)
	}

	// Should have --dangerously-allow-all for streaming mode
	foundAutonomy := false
	for _, arg := range args {
		if arg == "--dangerously-allow-all" {
			foundAutonomy = true
			break
		}
	}
	if !foundAutonomy {
		t.Errorf("Expected --dangerously-allow-all in args for streaming mode, got %v", args)
	}
}

func TestBuildArgs_NonClaudeTextMode(t *testing.T) {
	// Text mode should only affect claude and amp, not other agents
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "custom-agent",
		},
	}
	r := NewRunner(settings)

	args, _ := r.buildArgs("test prompt", 1, RunOptions{TextMode: true})

	// Should NOT have --output-format for non-claude agents
	for _, arg := range args {
		if arg == "--output-format" {
			t.Errorf("Should not have --output-format for non-claude agent, got %v", args)
		}
	}
	// Should NOT have --dangerously-allow-all for non-amp agents
	for _, arg := range args {
		if arg == "--dangerously-allow-all" {
			t.Errorf("Should not have --dangerously-allow-all for non-amp agent, got %v", args)
		}
	}
}

func TestBuildArgs_CodexStreamingMode(t *testing.T) {
	// Create .ralph directory for prompt file
	if err := os.MkdirAll(RalphDir, 0o755); err != nil {
		t.Fatalf("Failed to create .ralph directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(RalphDir) }()

	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "codex",
			Flags:   []string{"--custom"},
		},
		StreamAgentOutput: true,
	}
	r := NewRunner(settings)

	args, promptFile := r.buildArgs("test prompt", 1, RunOptions{})

	// Should have "e" subcommand for codex
	if len(args) < 1 || args[0] != "e" {
		t.Errorf("Expected first arg to be 'e', got %v", args)
	}

	// Should have --json flag
	foundJSON := false
	for _, arg := range args {
		if arg == "--json" {
			foundJSON = true
			break
		}
	}
	if !foundJSON {
		t.Errorf("Expected --json in args, got %v", args)
	}

	// Should have --full-auto flag
	foundFullAuto := false
	for _, arg := range args {
		if arg == "--full-auto" {
			foundFullAuto = true
			break
		}
	}
	if !foundFullAuto {
		t.Errorf("Expected --full-auto in args, got %v", args)
	}

	// Should have prompt file
	if promptFile == "" {
		t.Error("Expected prompt file for codex")
	}

	// Should include user flags
	foundCustom := false
	for _, arg := range args {
		if arg == "--custom" {
			foundCustom = true
			break
		}
	}
	if !foundCustom {
		t.Errorf("Expected --custom in args, got %v", args)
	}
}

func TestBuildArgs_CodexTextMode(t *testing.T) {
	// Create .ralph directory for prompt file
	if err := os.MkdirAll(RalphDir, 0o755); err != nil {
		t.Fatalf("Failed to create .ralph directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(RalphDir) }()

	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "codex",
		},
		StreamAgentOutput: true, // Would normally add --json, but text mode overrides
	}
	r := NewRunner(settings)

	outputFile := ".ralph/test_output.txt"
	args, promptFile := r.buildArgs("test prompt", 1, RunOptions{TextMode: true, OutputFile: outputFile})

	// Should have "e" subcommand for codex
	if len(args) < 1 || args[0] != "e" {
		t.Errorf("Expected first arg to be 'e', got %v", args)
	}

	// Should have --full-auto flag (text mode uses this for autonomy)
	foundFullAuto := false
	for _, arg := range args {
		if arg == "--full-auto" {
			foundFullAuto = true
			break
		}
	}
	if !foundFullAuto {
		t.Errorf("Expected --full-auto in args, got %v", args)
	}

	// Should NOT have --json flag in text mode
	for _, arg := range args {
		if arg == "--json" {
			t.Errorf("Should not have --json in text mode, got %v", args)
		}
	}

	// Should have -o flag with output file
	foundO := false
	for i, arg := range args {
		if arg == "-o" && i+1 < len(args) && args[i+1] == outputFile {
			foundO = true
			break
		}
	}
	if !foundO {
		t.Errorf("Expected -o %s in args, got %v", outputFile, args)
	}

	// Should have prompt file
	if promptFile == "" {
		t.Error("Expected prompt file for codex")
	}
}

func TestBuildArgs_CodexTextModeNoOutputFile(t *testing.T) {
	// Create .ralph directory for prompt file
	if err := os.MkdirAll(RalphDir, 0o755); err != nil {
		t.Fatalf("Failed to create .ralph directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(RalphDir) }()

	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "codex",
		},
	}
	r := NewRunner(settings)

	// Text mode without output file - should not add -o flag
	args, _ := r.buildArgs("test prompt", 1, RunOptions{TextMode: true})

	// Should NOT have -o flag when output file is empty
	for _, arg := range args {
		if arg == "-o" {
			t.Errorf("Should not have -o flag when no output file specified, got %v", args)
		}
	}
}

func TestBuildCmd_Basic(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "echo",
			Flags:   []string{"-n"},
		},
	}

	var stderr bytes.Buffer
	r := NewRunner(settings)
	r.Stderr = &stderr

	cmd, cleanup := r.buildCmd(context.Background(), "hello", 1, RunOptions{})
	defer cleanup()

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}
	// Command should use shell
	if cmd.Args[0] == "" {
		t.Error("Expected shell to be set")
	}
	// Stdin should be nil (non-interactive)
	if cmd.Stdin != nil {
		t.Error("Expected stdin to be nil")
	}
	// Should log the command
	if !bytes.Contains(stderr.Bytes(), []byte("[ralph] Agent command:")) {
		t.Error("Expected command to be logged to stderr")
	}
}

func TestBuildCmd_WithPromptFile(t *testing.T) {
	// Create .ralph directory for prompt file
	if err := os.MkdirAll(RalphDir, 0o755); err != nil {
		t.Fatalf("Failed to create .ralph directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(RalphDir) }()

	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "codex",
		},
	}

	var stderr bytes.Buffer
	r := NewRunner(settings)
	r.Stderr = &stderr

	_, cleanup := r.buildCmd(context.Background(), "test prompt", 1, RunOptions{})

	// Prompt file should exist before cleanup
	promptFile := ".ralph/prompt_001.txt"
	if _, err := os.Stat(promptFile); os.IsNotExist(err) {
		t.Error("Expected prompt file to be created")
	}

	// After cleanup, prompt file should be removed
	cleanup()
	if _, err := os.Stat(promptFile); !os.IsNotExist(err) {
		t.Error("Expected prompt file to be cleaned up")
	}
}

func TestCreateStreamProcessor_Disabled(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "claude",
		},
		StreamAgentOutput: false, // Streaming disabled
	}

	var stdout bytes.Buffer
	r := NewRunner(settings)
	r.Stdout = &stdout

	proc, logFile := r.createStreamProcessor()

	if proc != nil {
		t.Error("Expected nil processor when streaming disabled")
	}
	if logFile != nil {
		t.Error("Expected nil log file when streaming disabled")
	}
}

func TestCreateStreamProcessor_Enabled(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "claude",
		},
		StreamAgentOutput: true,
	}

	var stdout bytes.Buffer
	r := NewRunner(settings)
	r.Stdout = &stdout

	// Create .ralph directory for log file
	if err := os.MkdirAll(RalphDir, 0o755); err != nil {
		t.Fatalf("Failed to create .ralph directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(RalphDir) }()

	proc, logFile := r.createStreamProcessor()

	if proc == nil {
		t.Error("Expected non-nil processor when streaming enabled")
	}
	if logFile == nil {
		t.Error("Expected non-nil log file for claude")
	}

	// Cleanup
	if proc != nil {
		_ = proc.Close()
	}
	if logFile != nil {
		_ = logFile.Close()
	}
}

func TestCreateStreamProcessor_NonClaude(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "other-agent",
		},
		StreamAgentOutput: true,
	}

	var stdout bytes.Buffer
	r := NewRunner(settings)
	r.Stdout = &stdout

	proc, logFile := r.createStreamProcessor()

	// Processor returns nil for unknown agents (no structured output support)
	if proc != nil {
		t.Error("Expected nil processor for unknown agent without structured output support")
		_ = proc.Close()
	}
	// No log file for non-claude agents
	if logFile != nil {
		t.Error("Expected nil log file for non-claude agent")
	}
}

func TestConfigureOutput_StreamingWithProcessor(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "claude",
		},
		StreamAgentOutput: true,
	}

	var stdout, stderr bytes.Buffer
	r := NewRunner(settings)
	r.Stdout = &stdout
	r.Stderr = &stderr

	// Create .ralph directory for log file
	if err := os.MkdirAll(RalphDir, 0o755); err != nil {
		t.Fatalf("Failed to create .ralph directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(RalphDir) }()

	proc, logFile := r.createStreamProcessor()
	defer func() {
		if proc != nil {
			_ = proc.Close()
		}
		if logFile != nil {
			_ = logFile.Close()
		}
	}()

	cmd := &exec.Cmd{}
	var outputBuf bytes.Buffer
	r.configureOutput(cmd, &outputBuf, proc)

	// Stdout and stderr should be configured
	if cmd.Stdout == nil {
		t.Error("Expected stdout to be configured")
	}
	if cmd.Stderr == nil {
		t.Error("Expected stderr to be configured")
	}
}

func TestConfigureOutput_StreamingNoProcessor(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "unknown-agent",
		},
		StreamAgentOutput: true,
	}

	var stdout, stderr bytes.Buffer
	r := NewRunner(settings)
	r.Stdout = &stdout
	r.Stderr = &stderr

	cmd := &exec.Cmd{}
	var outputBuf bytes.Buffer
	r.configureOutput(cmd, &outputBuf, nil) // No processor

	// Stdout and stderr should be configured for raw streaming
	if cmd.Stdout == nil {
		t.Error("Expected stdout to be configured")
	}
	if cmd.Stderr == nil {
		t.Error("Expected stderr to be configured")
	}
}

func TestConfigureOutput_NoStreaming(t *testing.T) {
	settings := &config.Settings{
		Agent: config.AgentConfig{
			Command: "echo",
		},
		StreamAgentOutput: false,
	}

	r := NewRunner(settings)

	cmd := &exec.Cmd{}
	var outputBuf bytes.Buffer
	r.configureOutput(cmd, &outputBuf, nil)

	// Stdout should capture only (not stream)
	if cmd.Stdout == nil {
		t.Error("Expected stdout to be configured")
	}
	if cmd.Stderr == nil {
		t.Error("Expected stderr to be configured")
	}
}
