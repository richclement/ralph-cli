# Plan: Claude Code JSON Stream Parser

## Goal
Parse Claude Code's `--output-format stream-json` output in real-time, extract tool calls and text responses, and display meaningful progress to the CLI user.

## Requirements
- Streaming mode is a CLI flag that applies to all agents, not a settings-only toggle
- Auto-detect Claude and add `--output-format stream-json` flag
- Parse newline-delimited JSON as it streams
- Display tool calls (Read, Edit, Bash, etc.) and assistant text
- Surface tool errors and permission prompts to user
- Track cumulative cost throughout session
- Completion detection via JSON `result` message type

---

## Sample Claude stream-json Output

Reference payloads for parser development:

```json
{"type":"system","subtype":"init","session_id":"abc123","tools":["Read","Write","Edit","Bash"]}
{"type":"assistant","message":{"content":[{"type":"text","text":"I'll read the file first."}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tool_1","name":"Read","input":{"file_path":"/path/to/file.go"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tool_1","content":"package main\n..."}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tool_2","name":"Bash","input":{"command":"go test ./..."}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tool_2","content":"PASS\nok  \tpkg\t0.5s","is_error":false}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tool_3","name":"Edit","input":{"file_path":"/path/to/file.go","old_string":"foo","new_string":"bar"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tool_3","content":"","is_error":true,"error":"Permission denied"}]}}
{"type":"result","subtype":"success","result":"done","total_cost_usd":0.0234,"session_id":"abc123"}
```

**Key observations:**
- Messages are newline-delimited JSON objects
- `assistant` messages contain tool_use or text content
- `user` messages contain tool_result (success or error)
- `result` message signals completion with final cost
- No `<response>` tags in stream-json mode

---

## Implementation Steps

### Step 1: Define Agent Parser Interface [DONE]
**New file: `internal/stream/types.go`**

Define event types and common structures:

```go
package stream

import "time"

// EventType categorizes parsed events
type EventType int

const (
    EventToolStart EventType = iota  // Tool invocation began
    EventToolEnd                      // Tool completed (success or error)
    EventText                         // Assistant text output
    EventResult                       // Final result/completion
    EventProgress                     // Cost update, status change
    EventUnknown                      // Unrecognized message (for debugging)
)

func (t EventType) String() string {
    return [...]string{"ToolStart", "ToolEnd", "Text", "Result", "Progress", "Unknown"}[t]
}

// Event represents a parsed agent output event (agent-agnostic)
type Event struct {
    Type       EventType
    Timestamp  time.Time // When event was received

    // Tool events
    ToolName   string    // e.g., "Read", "Edit", "Bash"
    ToolID     string    // Correlation ID for start/end matching
    ToolInput  string    // Summary of input (file path, command, etc.)
    ToolOutput string    // Summary of output (for ToolEnd)
    ToolError  string    // Error message if tool failed

    // Text events
    Text       string    // Assistant text output

    // Result events
    Result     string    // Final result/completion text
    IsComplete bool      // Signals end of stream

    // Cost tracking
    Cost       float64   // Cumulative cost at this point
    CostDelta  float64   // Cost of this specific operation
}

// IsError returns true if this event represents a failure
func (e *Event) IsError() bool {
    return e.ToolError != ""
}
```

**New file: `internal/stream/parser.go`**

Parser interface and factory:

```go
package stream

import (
    "path/filepath"
    "strings"
)

// Parser interface - each agent implements this
type Parser interface {
    // Parse decodes raw bytes and returns parsed events
    // Returns nil event for non-displayable messages
    // May return multiple events for a single message
    Parse(data []byte) ([]*Event, error)

    // Name returns the agent name for display
    Name() string
}

// ParserFor returns the appropriate parser for an agent command
// Returns nil if agent doesn't support structured output parsing
func ParserFor(agentCommand string) Parser {
    name := strings.ToLower(filepath.Base(agentCommand))
    name = strings.TrimSuffix(name, ".exe")

    switch name {
    case "claude":
        return NewClaudeParser()
    case "codex":
        return NewCodexParser()
    case "amp":
        return NewAmpParser()
    default:
        return nil
    }
}

// OutputFlags returns agent-specific flags for structured output
// Returns nil if agent doesn't support structured output
func OutputFlags(agentCommand string) []string {
    name := strings.ToLower(filepath.Base(agentCommand))
    name = strings.TrimSuffix(name, ".exe")

    switch name {
    case "claude":
        return []string{"--output-format", "stream-json"}
    // case "codex": return []string{"--output", "json"} // when known
    // case "amp": return []string{"--format", "json"} // when known
    default:
        return nil
    }
}
```

### Step 2: Implement Claude Code Parser [DONE]
**New file: `internal/stream/claude.go`**

Claude-specific JSON structures and parser:

```go
package stream

import (
    "encoding/json"
    "fmt"
    "time"
)

// Claude stream-json raw message types
type claudeRawMessage struct {
    Type      string         `json:"type"`
    Subtype   string         `json:"subtype,omitempty"`
    SessionID string         `json:"session_id,omitempty"`
    Message   *claudeMessage `json:"message,omitempty"`
    Result    string         `json:"result,omitempty"`
    Cost      float64        `json:"total_cost_usd,omitempty"`
}

type claudeMessage struct {
    Content []claudeContent `json:"content"`
}

type claudeContent struct {
    Type      string                 `json:"type"`
    ID        string                 `json:"id,omitempty"`
    ToolUseID string                 `json:"tool_use_id,omitempty"`
    Name      string                 `json:"name,omitempty"`
    Text      string                 `json:"text,omitempty"`
    Content   string                 `json:"content,omitempty"`
    Input     map[string]interface{} `json:"input,omitempty"`
    IsError   bool                   `json:"is_error,omitempty"`
    Error     string                 `json:"error,omitempty"`
}

// ClaudeParser implements Parser for Claude Code stream-json
type ClaudeParser struct {
    cumulativeCost float64
    activeTools    map[string]time.Time // toolID -> start time
}

func NewClaudeParser() *ClaudeParser {
    return &ClaudeParser{
        activeTools: make(map[string]time.Time),
    }
}

func (p *ClaudeParser) Name() string {
    return "claude"
}

func (p *ClaudeParser) Parse(data []byte) ([]*Event, error) {
    var raw claudeRawMessage
    if err := json.Unmarshal(data, &raw); err != nil {
        return nil, fmt.Errorf("json unmarshal: %w", err)
    }

    now := time.Now()

    switch raw.Type {
    case "assistant":
        return p.parseAssistant(&raw, now)
    case "user":
        return p.parseToolResult(&raw, now)
    case "result":
        costDelta := raw.Cost - p.cumulativeCost
        p.cumulativeCost = raw.Cost
        return []*Event{{
            Type:       EventResult,
            Timestamp:  now,
            Result:     raw.Result,
            Cost:       raw.Cost,
            CostDelta:  costDelta,
            IsComplete: true,
        }}, nil
    case "system":
        // Could extract session info, tools list for verbose mode
        return []*Event{{
            Type:      EventProgress,
            Timestamp: now,
            Text:      fmt.Sprintf("session: %s", raw.SessionID),
        }}, nil
    default:
        // Log unknown types for debugging, don't fail
        return []*Event{{
            Type:      EventUnknown,
            Timestamp: now,
            Text:      fmt.Sprintf("unknown message type: %s", raw.Type),
        }}, nil
    }
}

func (p *ClaudeParser) parseAssistant(raw *claudeRawMessage, now time.Time) ([]*Event, error) {
    if raw.Message == nil {
        return nil, nil
    }

    var events []*Event
    for _, c := range raw.Message.Content {
        switch c.Type {
        case "tool_use":
            p.activeTools[c.ID] = now
            events = append(events, &Event{
                Type:      EventToolStart,
                Timestamp: now,
                ToolName:  c.Name,
                ToolID:    c.ID,
                ToolInput: extractToolInput(c.Name, c.Input),
            })
        case "text":
            if c.Text != "" {
                events = append(events, &Event{
                    Type:      EventText,
                    Timestamp: now,
                    Text:      c.Text,
                })
            }
        }
    }
    return events, nil
}

func (p *ClaudeParser) parseToolResult(raw *claudeRawMessage, now time.Time) ([]*Event, error) {
    if raw.Message == nil {
        return nil, nil
    }

    var events []*Event
    for _, c := range raw.Message.Content {
        if c.Type != "tool_result" {
            continue
        }

        event := &Event{
            Type:      EventToolEnd,
            Timestamp: now,
            ToolID:    c.ToolUseID,
        }

        // Calculate duration if we tracked the start
        if startTime, ok := p.activeTools[c.ToolUseID]; ok {
            delete(p.activeTools, c.ToolUseID)
            _ = now.Sub(startTime) // Duration available for future use
        }

        if c.IsError {
            event.ToolError = c.Error
            if event.ToolError == "" {
                event.ToolError = c.Content // Sometimes error is in content
            }
        } else {
            event.ToolOutput = truncate(c.Content, 100)
        }

        events = append(events, event)
    }
    return events, nil
}

// extractToolInput summarizes tool input for display
func extractToolInput(name string, input map[string]interface{}) string {
    switch name {
    case "Read":
        if path, ok := input["file_path"].(string); ok {
            return path
        }
    case "Write", "Edit":
        if path, ok := input["file_path"].(string); ok {
            return path
        }
    case "Bash":
        if cmd, ok := input["command"].(string); ok {
            return truncate(cmd, 60)
        }
        if desc, ok := input["description"].(string); ok {
            return truncate(desc, 60)
        }
    case "Glob", "Grep":
        if pattern, ok := input["pattern"].(string); ok {
            return truncate(pattern, 40)
        }
    case "Task":
        if desc, ok := input["description"].(string); ok {
            return truncate(desc, 40)
        }
    case "WebFetch", "WebSearch":
        if url, ok := input["url"].(string); ok {
            return truncate(url, 50)
        }
        if query, ok := input["query"].(string); ok {
            return truncate(query, 50)
        }
    }
    return ""
}

func truncate(s string, max int) string {
    if len(s) <= max {
        return s
    }
    return s[:max-3] + "..."
}
```

### Step 3: Stub Parsers for Other Agents [DONE]
**New file: `internal/stream/codex.go`**

```go
package stream

// CodexParser implements Parser for OpenAI Codex CLI
// TODO: implement when Codex output format is documented
type CodexParser struct{}

func NewCodexParser() *CodexParser {
    return &CodexParser{}
}

func (p *CodexParser) Name() string {
    return "codex"
}

func (p *CodexParser) Parse(data []byte) ([]*Event, error) {
    // Pass through for now - returns nil to skip display
    return nil, nil
}
```

**New file: `internal/stream/amp.go`**

```go
package stream

// AmpParser implements Parser for Sourcegraph Amp
// TODO: implement when Amp output format is documented
type AmpParser struct{}

func NewAmpParser() *AmpParser {
    return &AmpParser{}
}

func (p *AmpParser) Name() string {
    return "amp"
}

func (p *AmpParser) Parse(data []byte) ([]*Event, error) {
    // Pass through for now - returns nil to skip display
    return nil, nil
}
```

### Step 4: Create Stream Formatter [DONE]
**New file: `internal/stream/formatter.go`**

Format parsed events for CLI display:

```go
package stream

import (
    "fmt"
    "io"
    "strings"
    "sync"

    "github.com/fatih/color"
    "golang.org/x/term"
)

// FormatterConfig controls output behavior
type FormatterConfig struct {
    AgentName    string // Display name, e.g., "claude"
    ShowText     bool   // Show assistant text (false = tools only)
    ShowProgress bool   // Show progress/unknown events
    UseColor     bool   // Enable ANSI colors
    Verbose      bool   // Show extra details (tool output, durations)
}

// DefaultFormatterConfig returns sensible defaults
func DefaultFormatterConfig(agentName string) FormatterConfig {
    // Auto-detect color support
    useColor := term.IsTerminal(1) // fd 1 = stdout

    return FormatterConfig{
        AgentName:    agentName,
        ShowText:     true,
        ShowProgress: false,
        UseColor:     useColor,
        Verbose:      false,
    }
}

// Formatter formats events for CLI display
type Formatter struct {
    out    io.Writer
    mu     sync.Mutex // Protect concurrent writes
    config FormatterConfig

    // Colorizers (no-op if color disabled)
    prefixColor *color.Color
    toolColor   *color.Color
    textColor   *color.Color
    errorColor  *color.Color
    costColor   *color.Color
}

// NewFormatter creates a formatter with the given config
func NewFormatter(out io.Writer, config FormatterConfig) *Formatter {
    f := &Formatter{
        out:    out,
        config: config,
    }

    if config.UseColor {
        f.prefixColor = color.New(color.FgCyan)
        f.toolColor = color.New(color.FgYellow)
        f.textColor = color.New(color.FgWhite)
        f.errorColor = color.New(color.FgRed, color.Bold)
        f.costColor = color.New(color.FgGreen)
    } else {
        // No-op colors
        noColor := color.New()
        f.prefixColor = noColor
        f.toolColor = noColor
        f.textColor = noColor
        f.errorColor = noColor
        f.costColor = noColor
    }

    return f
}

// FormatEvent writes a formatted event to output
func (f *Formatter) FormatEvent(e *Event) {
    if e == nil {
        return
    }

    f.mu.Lock()
    defer f.mu.Unlock()

    prefix := fmt.Sprintf("[%s] ", f.config.AgentName)

    switch e.Type {
    case EventToolStart:
        line := fmt.Sprintf("%s%s", e.ToolName, f.formatInput(e.ToolInput))
        f.writeLine(f.prefixColor, prefix, f.toolColor, line)

    case EventToolEnd:
        if e.IsError() {
            line := fmt.Sprintf("ERROR: %s", e.ToolError)
            f.writeLine(f.prefixColor, prefix, f.errorColor, line)
        } else if f.config.Verbose && e.ToolOutput != "" {
            line := fmt.Sprintf("  -> %s", e.ToolOutput)
            f.writeLine(f.prefixColor, prefix, f.textColor, line)
        }

    case EventText:
        if f.config.ShowText && e.Text != "" {
            // Truncate long text, show first line
            text := firstLine(e.Text, 80)
            f.writeLine(f.prefixColor, prefix, f.textColor, fmt.Sprintf("%q", text))
        }

    case EventResult:
        line := fmt.Sprintf("Complete (cost: $%.4f)", e.Cost)
        f.writeLine(f.prefixColor, prefix, f.costColor, line)

    case EventProgress, EventUnknown:
        if f.config.ShowProgress {
            f.writeLine(f.prefixColor, prefix, f.textColor, e.Text)
        }
    }
}

func (f *Formatter) formatInput(input string) string {
    if input == "" {
        return ""
    }
    return ": " + input
}

func (f *Formatter) writeLine(prefixColor *color.Color, prefix string, contentColor *color.Color, content string) {
    prefixColor.Fprint(f.out, prefix)
    contentColor.Fprintln(f.out, content)
}

func firstLine(s string, max int) string {
    if idx := strings.Index(s, "\n"); idx != -1 {
        s = s[:idx]
    }
    if len(s) > max {
        return s[:max-3] + "..."
    }
    return s
}
```

### Step 5: Create Stream Processor [DONE]
**New file: `internal/stream/processor.go`**

Use `bufio.Reader` over an `io.Pipe` to handle arbitrary JSON object sizes without line limits:

```go
package stream

import (
    "bufio"
    "bytes"
    "encoding/json"
    "io"
    "log"
    "sync"
    "sync/atomic"
    "time"
)

// Processor decodes JSON stream and formats events
type Processor struct {
    parser     Parser
    formatter  *Formatter
    pipeReader *io.PipeReader
    pipeWriter *io.PipeWriter
    done       chan struct{}
    closeOnce  sync.Once

    // Observability
    lastActivity atomic.Value // time.Time
    eventCount   atomic.Int64
    errorCount   atomic.Int64
    debugLog     *log.Logger // Optional, for verbose debugging
}

// NewProcessor creates a processor for the given agent command
// Returns nil if agent doesn't support structured output parsing
func NewProcessor(agentCommand string, formatter *Formatter, debugLog *log.Logger) *Processor {
    // Only create a processor if the agent has structured output flags.
    // Otherwise, fall back to raw streaming output.
    if flags := OutputFlags(agentCommand); len(flags) == 0 {
        return nil
    }

    parser := ParserFor(agentCommand)
    if parser == nil {
        return nil
    }

    pr, pw := io.Pipe()
    p := &Processor{
        parser:     parser,
        formatter:  formatter,
        pipeReader: pr,
        pipeWriter: pw,
        done:       make(chan struct{}),
        debugLog:   debugLog,
    }
    p.lastActivity.Store(time.Now())
    go p.decodeLoop()
    return p
}

func (p *Processor) decodeLoop() {
    defer close(p.done)

    reader := bufio.NewReader(p.pipeReader)

    for {
        line, err := reader.ReadBytes('\n')
        if len(line) > 0 {
            trimmed := bytes.TrimSpace(line)
            if len(trimmed) > 0 {
                p.lastActivity.Store(time.Now())

                if !json.Valid(trimmed) {
                    p.errorCount.Add(1)
                    if p.debugLog != nil {
                        p.debugLog.Printf("invalid JSON: %s", truncate(string(trimmed), 100))
                    }
                } else {
                    events, parseErr := p.parser.Parse(trimmed)
                    if parseErr != nil {
                        p.errorCount.Add(1)
                        if p.debugLog != nil {
                            p.debugLog.Printf("parse error: %v (raw: %s)", parseErr, truncate(string(trimmed), 100))
                        }
                    } else {
                        for _, event := range events {
                            if event != nil {
                                p.eventCount.Add(1)
                                p.formatter.FormatEvent(event)
                            }
                        }
                    }
                }
            }
        }
        if err != nil {
            if err == io.EOF {
                return
            }
            p.errorCount.Add(1)
            if p.debugLog != nil {
                p.debugLog.Printf("reader error: %v", err)
            }
            return
        }
    }
}

// Write implements io.Writer - pipe bytes to decoder
func (p *Processor) Write(data []byte) (int, error) {
    return p.pipeWriter.Write(data)
}

// Close signals EOF and waits for decoder to finish
func (p *Processor) Close() error {
    p.closeOnce.Do(func() {
        p.pipeWriter.Close()
    })
    <-p.done
    return nil
}

// LastActivity returns when the last event was processed
// Useful for detecting stuck/hung agents
func (p *Processor) LastActivity() time.Time {
    if t, ok := p.lastActivity.Load().(time.Time); ok {
        return t
    }
    return time.Time{}
}

// Stats returns processing statistics
func (p *Processor) Stats() (events, errors int64) {
    return p.eventCount.Load(), p.errorCount.Load()
}
```

**Why bufio.Reader over bufio.Scanner:**
- No fixed line length limits (Scanner defaults to 64KB)
- Handles large tool results (file contents, grep output)
- Preserves streaming resilience by skipping malformed lines
- Straightforward newline-delimited parsing for stream-json output

### Step 6: Modify Agent Runner [DONE]
**File: `internal/agent/agent.go`**

Inject agent-specific flags and wire up processor:

```go
// In buildCommand or equivalent method
func (r *Runner) buildCommand(ctx context.Context, prompt string) *exec.Cmd {
    // Copy args so we don't mutate settings across iterations.
    args := append([]string{}, r.Settings.Agent.Args...)

    // Add structured output flags if streaming is enabled
    if r.Settings.StreamAgentOutput {
        if flags := stream.OutputFlags(r.Settings.Agent.Command); flags != nil {
            args = append(args, flags...)
        }
    }

    // ... rest of command building
}

// In Run method
func (r *Runner) Run(ctx context.Context, prompt string, iteration int) (string, error) {
    // ...

    var proc *stream.Processor
    if r.Settings.StreamAgentOutput {
        config := stream.DefaultFormatterConfig(filepath.Base(r.Settings.Agent.Command))
        formatter := stream.NewFormatter(r.Stdout, config)

        // debugLog is nil unless --verbose flag set
        var debugLog *log.Logger
        if r.Settings.Verbose {
            debugLog = log.New(r.Stderr, "[stream-debug] ", log.LstdFlags)
        }

        proc = stream.NewProcessor(r.Settings.Agent.Command, formatter, debugLog)
    }

    if proc != nil {
        defer proc.Close()
        // Structured output: parse JSON, format events
        cmd.Stdout = io.MultiWriter(&outputBuf, proc)
        // Still stream stderr to user for prompts/errors
        cmd.Stderr = io.MultiWriter(&outputBuf, r.Stderr)
    } else {
        // No parser available: raw streaming fallback
        cmd.Stdout = io.MultiWriter(&outputBuf, r.Stdout)
        cmd.Stderr = io.MultiWriter(&outputBuf, r.Stderr)
    }

    // ... run command

    // Log stats if verbose
    if proc != nil && r.Settings.Verbose {
        events, errors := proc.Stats()
        log.Printf("stream stats: %d events, %d errors", events, errors)
    }
}
```

### Step 7: Update Completion Detection [DONE]
**File: `internal/response/response.go`**

Add JSON result extraction - note that with `--output-format stream-json`, there are no `<response>` tags:

```go
import (
    "encoding/json"
    "strings"
)

// resultMessage matches Claude's result JSON structure
type resultMessage struct {
    Type   string `json:"type"`
    Result string `json:"result"`
}

// ExtractFromJSON finds the last {"type":"result"} message and extracts the result field
func ExtractFromJSON(output string) (string, bool) {
    // Scan backwards for efficiency - result is always last
    lines := strings.Split(output, "\n")
    for i := len(lines) - 1; i >= 0; i-- {
        line := strings.TrimSpace(lines[i])
        if line == "" {
            continue
        }

        // Quick check before parsing
        if !strings.Contains(line, `"type":"result"`) {
            continue
        }

        var msg resultMessage
        if err := json.Unmarshal([]byte(line), &msg); err != nil {
            continue
        }

        if msg.Type == "result" {
            return msg.Result, true
        }
    }
    return "", false
}

// IsComplete checks if agent output indicates completion
// For stream-json mode: checks JSON result
// For text mode: falls back to <response> regex
func IsComplete(output, completionResponse string) bool {
    // Try JSON extraction first (stream-json mode)
    if result, found := ExtractFromJSON(output); found {
        return strings.EqualFold(strings.TrimSpace(result), strings.TrimSpace(completionResponse))
    }

    // Fall back to <response> regex if no JSON result found
    if resp, found := ExtractResponse(output); found {
        return strings.EqualFold(strings.TrimSpace(resp), strings.TrimSpace(completionResponse))
    }

    return false
}
```

### Step 8: Add Tests [DONE]

**New file: `internal/stream/parser_test.go`**

```go
package stream

import (
    "testing"
)

func TestParserFor(t *testing.T) {
    tests := []struct {
        command  string
        wantName string
        wantNil  bool
    }{
        {"claude", "claude", false},
        {"/usr/local/bin/claude", "claude", false},
        {"claude.exe", "claude", false},
        {"codex", "codex", false},
        {"amp", "amp", false},
        {"unknown-agent", "", true},
        {"", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.command, func(t *testing.T) {
            p := ParserFor(tt.command)
            if tt.wantNil {
                if p != nil {
                    t.Errorf("ParserFor(%q) = %v, want nil", tt.command, p)
                }
                return
            }
            if p == nil {
                t.Fatalf("ParserFor(%q) = nil, want parser", tt.command)
            }
            if got := p.Name(); got != tt.wantName {
                t.Errorf("parser.Name() = %q, want %q", got, tt.wantName)
            }
        })
    }
}

func TestOutputFlags(t *testing.T) {
    tests := []struct {
        command string
        want    []string
    }{
        {"claude", []string{"--output-format", "stream-json"}},
        {"/path/to/claude", []string{"--output-format", "stream-json"}},
        {"unknown", nil},
    }

    for _, tt := range tests {
        t.Run(tt.command, func(t *testing.T) {
            got := OutputFlags(tt.command)
            if len(got) != len(tt.want) {
                t.Errorf("OutputFlags(%q) = %v, want %v", tt.command, got, tt.want)
            }
        })
    }
}
```

**New file: `internal/stream/claude_test.go`**

```go
package stream

import (
    "testing"
)

func TestClaudeParser_ToolUse(t *testing.T) {
    p := NewClaudeParser()

    input := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tool_1","name":"Read","input":{"file_path":"/path/to/file.go"}}]}}`

    events, err := p.Parse([]byte(input))
    if err != nil {
        t.Fatalf("Parse() error = %v", err)
    }

    if len(events) != 1 {
        t.Fatalf("got %d events, want 1", len(events))
    }

    e := events[0]
    if e.Type != EventToolStart {
        t.Errorf("Type = %v, want EventToolStart", e.Type)
    }
    if e.ToolName != "Read" {
        t.Errorf("ToolName = %q, want %q", e.ToolName, "Read")
    }
    if e.ToolInput != "/path/to/file.go" {
        t.Errorf("ToolInput = %q, want %q", e.ToolInput, "/path/to/file.go")
    }
}

func TestClaudeParser_ToolResult(t *testing.T) {
    p := NewClaudeParser()

    // First, register a tool start
    startInput := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tool_1","name":"Bash","input":{"command":"ls"}}]}}`
    _, _ = p.Parse([]byte(startInput))

    // Then parse the result
    resultInput := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tool_1","content":"file1.go\nfile2.go"}]}}`

    events, err := p.Parse([]byte(resultInput))
    if err != nil {
        t.Fatalf("Parse() error = %v", err)
    }

    if len(events) != 1 {
        t.Fatalf("got %d events, want 1", len(events))
    }

    e := events[0]
    if e.Type != EventToolEnd {
        t.Errorf("Type = %v, want EventToolEnd", e.Type)
    }
    if e.ToolID != "tool_1" {
        t.Errorf("ToolID = %q, want %q", e.ToolID, "tool_1")
    }
}

func TestClaudeParser_ToolError(t *testing.T) {
    p := NewClaudeParser()

    input := `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tool_1","content":"","is_error":true,"error":"Permission denied"}]}}`

    events, err := p.Parse([]byte(input))
    if err != nil {
        t.Fatalf("Parse() error = %v", err)
    }

    if len(events) != 1 {
        t.Fatalf("got %d events, want 1", len(events))
    }

    e := events[0]
    if !e.IsError() {
        t.Error("expected IsError() = true")
    }
    if e.ToolError != "Permission denied" {
        t.Errorf("ToolError = %q, want %q", e.ToolError, "Permission denied")
    }
}

func TestClaudeParser_Text(t *testing.T) {
    p := NewClaudeParser()

    input := `{"type":"assistant","message":{"content":[{"type":"text","text":"I'll help you with that."}]}}`

    events, err := p.Parse([]byte(input))
    if err != nil {
        t.Fatalf("Parse() error = %v", err)
    }

    if len(events) != 1 {
        t.Fatalf("got %d events, want 1", len(events))
    }

    e := events[0]
    if e.Type != EventText {
        t.Errorf("Type = %v, want EventText", e.Type)
    }
    if e.Text != "I'll help you with that." {
        t.Errorf("Text = %q, want %q", e.Text, "I'll help you with that.")
    }
}

func TestClaudeParser_Result(t *testing.T) {
    p := NewClaudeParser()

    input := `{"type":"result","subtype":"success","result":"done","total_cost_usd":0.0234}`

    events, err := p.Parse([]byte(input))
    if err != nil {
        t.Fatalf("Parse() error = %v", err)
    }

    if len(events) != 1 {
        t.Fatalf("got %d events, want 1", len(events))
    }

    e := events[0]
    if e.Type != EventResult {
        t.Errorf("Type = %v, want EventResult", e.Type)
    }
    if e.Result != "done" {
        t.Errorf("Result = %q, want %q", e.Result, "done")
    }
    if e.Cost != 0.0234 {
        t.Errorf("Cost = %v, want %v", e.Cost, 0.0234)
    }
    if !e.IsComplete {
        t.Error("expected IsComplete = true")
    }
}

func TestClaudeParser_UnknownType(t *testing.T) {
    p := NewClaudeParser()

    input := `{"type":"future_type","data":"something"}`

    events, err := p.Parse([]byte(input))
    if err != nil {
        t.Fatalf("Parse() error = %v", err)
    }

    // Should return unknown event, not fail
    if len(events) != 1 {
        t.Fatalf("got %d events, want 1", len(events))
    }

    if events[0].Type != EventUnknown {
        t.Errorf("Type = %v, want EventUnknown", events[0].Type)
    }
}

func TestClaudeParser_MalformedJSON(t *testing.T) {
    p := NewClaudeParser()

    input := `{not valid json`

    _, err := p.Parse([]byte(input))
    if err == nil {
        t.Error("expected error for malformed JSON")
    }
}

func TestExtractToolInput(t *testing.T) {
    tests := []struct {
        name  string
        input map[string]interface{}
        want  string
    }{
        {"Read", map[string]interface{}{"file_path": "/path/to/file"}, "/path/to/file"},
        {"Edit", map[string]interface{}{"file_path": "/path/to/file"}, "/path/to/file"},
        {"Bash", map[string]interface{}{"command": "go test ./..."}, "go test ./..."},
        {"Bash description", map[string]interface{}{"description": "Run tests"}, "Run tests"},
        {"Grep", map[string]interface{}{"pattern": "TODO"}, "TODO"},
        {"Unknown", map[string]interface{}{"foo": "bar"}, ""},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := extractToolInput(tt.name, tt.input)
            if got != tt.want {
                t.Errorf("extractToolInput(%q, %v) = %q, want %q", tt.name, tt.input, got, tt.want)
            }
        })
    }
}
```

**New file: `internal/stream/formatter_test.go`**

```go
package stream

import (
    "bytes"
    "strings"
    "testing"
)

func TestFormatter_ToolStart(t *testing.T) {
    var buf bytes.Buffer
    config := FormatterConfig{AgentName: "claude", UseColor: false}
    f := NewFormatter(&buf, config)

    f.FormatEvent(&Event{
        Type:      EventToolStart,
        ToolName:  "Read",
        ToolInput: "/path/to/file.go",
    })

    got := buf.String()
    if !strings.Contains(got, "[claude]") {
        t.Errorf("output missing prefix: %q", got)
    }
    if !strings.Contains(got, "Read") {
        t.Errorf("output missing tool name: %q", got)
    }
    if !strings.Contains(got, "/path/to/file.go") {
        t.Errorf("output missing tool input: %q", got)
    }
}

func TestFormatter_Error(t *testing.T) {
    var buf bytes.Buffer
    config := FormatterConfig{AgentName: "claude", UseColor: false}
    f := NewFormatter(&buf, config)

    f.FormatEvent(&Event{
        Type:      EventToolEnd,
        ToolError: "Permission denied",
    })

    got := buf.String()
    if !strings.Contains(got, "ERROR") {
        t.Errorf("output missing ERROR: %q", got)
    }
    if !strings.Contains(got, "Permission denied") {
        t.Errorf("output missing error message: %q", got)
    }
}

func TestFormatter_Result(t *testing.T) {
    var buf bytes.Buffer
    config := FormatterConfig{AgentName: "claude", UseColor: false}
    f := NewFormatter(&buf, config)

    f.FormatEvent(&Event{
        Type: EventResult,
        Cost: 0.0234,
    })

    got := buf.String()
    if !strings.Contains(got, "Complete") {
        t.Errorf("output missing Complete: %q", got)
    }
    if !strings.Contains(got, "0.0234") {
        t.Errorf("output missing cost: %q", got)
    }
}

func TestFormatter_TextDisabled(t *testing.T) {
    var buf bytes.Buffer
    config := FormatterConfig{AgentName: "claude", ShowText: false, UseColor: false}
    f := NewFormatter(&buf, config)

    f.FormatEvent(&Event{
        Type: EventText,
        Text: "Some assistant text",
    })

    if buf.Len() > 0 {
        t.Errorf("expected no output when ShowText=false, got: %q", buf.String())
    }
}

func TestFormatter_ConcurrentWrites(t *testing.T) {
    var buf bytes.Buffer
    config := FormatterConfig{AgentName: "claude", UseColor: false}
    f := NewFormatter(&buf, config)

    // Simulate concurrent writes
    done := make(chan struct{})
    for i := 0; i < 10; i++ {
        go func(n int) {
            defer func() { done <- struct{}{} }()
            for j := 0; j < 100; j++ {
                f.FormatEvent(&Event{
                    Type:      EventToolStart,
                    ToolName:  "Read",
                    ToolInput: "/path",
                })
            }
        }(i)
    }

    for i := 0; i < 10; i++ {
        <-done
    }

    // Just verify no panic - output may be interleaved but shouldn't crash
    lines := strings.Split(buf.String(), "\n")
    if len(lines) < 1000 {
        t.Errorf("expected ~1000 lines, got %d", len(lines))
    }
}
```

**New file: `internal/stream/processor_test.go`**

```go
package stream

import (
    "bytes"
    "strings"
    "testing"
    "time"
)

func TestProcessor_Integration(t *testing.T) {
    var buf bytes.Buffer
    config := FormatterConfig{AgentName: "claude", UseColor: false, ShowText: true}
    formatter := NewFormatter(&buf, config)

    proc := NewProcessor("claude", formatter, nil)
    if proc == nil {
        t.Fatal("NewProcessor returned nil")
    }

    // Write sample Claude output
    messages := []string{
        `{"type":"system","subtype":"init","session_id":"test"}`,
        `{"type":"assistant","message":{"content":[{"type":"text","text":"Let me help."}]}}`,
        `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"/test.go"}}]}}`,
        `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":"package main"}]}}`,
        `{"type":"result","subtype":"success","result":"done","total_cost_usd":0.01}`,
    }

    for _, msg := range messages {
        _, err := proc.Write([]byte(msg + "\n"))
        if err != nil {
            t.Fatalf("Write error: %v", err)
        }
    }

    // Close and wait
    proc.Close()

    output := buf.String()

    // Verify output contains expected elements
    if !strings.Contains(output, "Read") {
        t.Errorf("output missing tool name: %q", output)
    }
    if !strings.Contains(output, "/test.go") {
        t.Errorf("output missing file path: %q", output)
    }
    if !strings.Contains(output, "Complete") {
        t.Errorf("output missing completion: %q", output)
    }

    // Check stats
    events, errors := proc.Stats()
    if events < 3 {
        t.Errorf("expected at least 3 events, got %d", events)
    }
    if errors != 0 {
        t.Errorf("expected 0 errors, got %d", errors)
    }
}

func TestProcessor_MalformedJSON(t *testing.T) {
    var buf bytes.Buffer
    config := FormatterConfig{AgentName: "claude", UseColor: false}
    formatter := NewFormatter(&buf, config)

    proc := NewProcessor("claude", formatter, nil)

    // Write mix of valid and invalid JSON
    proc.Write([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Valid"}]}}` + "\n"))
    proc.Write([]byte(`{invalid json}` + "\n"))
    proc.Write([]byte(`{"type":"result","result":"done","total_cost_usd":0.01}` + "\n"))

    proc.Close()

    // Should still process valid messages
    output := buf.String()
    if !strings.Contains(output, "Complete") {
        t.Errorf("failed to recover from malformed JSON: %q", output)
    }

    _, errors := proc.Stats()
    if errors == 0 {
        t.Error("expected error count > 0 for malformed JSON")
    }
}

func TestProcessor_LastActivity(t *testing.T) {
    var buf bytes.Buffer
    config := FormatterConfig{AgentName: "claude", UseColor: false}
    formatter := NewFormatter(&buf, config)

    proc := NewProcessor("claude", formatter, nil)

    before := time.Now()
    time.Sleep(10 * time.Millisecond)

    proc.Write([]byte(`{"type":"result","result":"done","total_cost_usd":0.01}` + "\n"))

    time.Sleep(10 * time.Millisecond)
    proc.Close()

    lastActivity := proc.LastActivity()
    if lastActivity.Before(before) {
        t.Errorf("LastActivity %v should be after %v", lastActivity, before)
    }
}

func TestProcessor_UnknownAgent(t *testing.T) {
    var buf bytes.Buffer
    config := FormatterConfig{AgentName: "unknown", UseColor: false}
    formatter := NewFormatter(&buf, config)

    proc := NewProcessor("unknown-agent", formatter, nil)
    if proc != nil {
        t.Error("expected nil processor for unknown agent")
    }
}
```

**Update: `internal/response/response_test.go`**

```go
// Add these tests to existing file

func TestExtractFromJSON(t *testing.T) {
    tests := []struct {
        name   string
        output string
        want   string
        found  bool
    }{
        {
            name:   "valid result",
            output: `{"type":"system"}\n{"type":"result","result":"done","total_cost_usd":0.01}`,
            want:   "done",
            found:  true,
        },
        {
            name:   "result in middle",
            output: "line1\n{\"type\":\"result\",\"result\":\"complete\"}\nline3",
            want:   "complete",
            found:  true,
        },
        {
            name:   "no result",
            output: `{"type":"assistant"}\n{"type":"user"}`,
            want:   "",
            found:  false,
        },
        {
            name:   "malformed json",
            output: `{not json}\n{"type":"result"`,
            want:   "",
            found:  false,
        },
        {
            name:   "empty output",
            output: "",
            want:   "",
            found:  false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, found := ExtractFromJSON(tt.output)
            if found != tt.found {
                t.Errorf("ExtractFromJSON() found = %v, want %v", found, tt.found)
            }
            if got != tt.want {
                t.Errorf("ExtractFromJSON() = %q, want %q", got, tt.want)
            }
        })
    }
}

func TestIsComplete_JSON(t *testing.T) {
    output := `{"type":"assistant"}\n{"type":"result","result":"done"}`

    if !IsComplete(output, "done") {
        t.Error("IsComplete should return true for matching result")
    }

    if IsComplete(output, "other") {
        t.Error("IsComplete should return false for non-matching result")
    }
}

func TestIsComplete_Fallback(t *testing.T) {
    // No JSON result, should fall back to <response> regex
    output := "Some output\n<response>completed</response>"

    if !IsComplete(output, "completed") {
        t.Error("IsComplete should fall back to <response> extraction")
    }
}
```

---

## Files to Create/Modify

| File | Action |
|------|--------|
| `internal/stream/types.go` | Create - Event types and structures |
| `internal/stream/parser.go` | Create - Parser interface + factory + OutputFlags |
| `internal/stream/claude.go` | Create - Claude Code parser implementation |
| `internal/stream/codex.go` | Create - Codex parser stub |
| `internal/stream/amp.go` | Create - Amp parser stub |
| `internal/stream/formatter.go` | Create - Event formatter for CLI with color support |
| `internal/stream/processor.go` | Create - newline-delimited JSON processor with observability |
| `internal/stream/parser_test.go` | Create |
| `internal/stream/claude_test.go` | Create |
| `internal/stream/formatter_test.go` | Create |
| `internal/stream/processor_test.go` | Create |
| `internal/agent/agent.go` | Modify - wire up processor, inject flags |
| `internal/response/response.go` | Modify - add JSON extraction |
| `internal/response/response_test.go` | Update - add JSON extraction tests |

---

## Example Output

When running `ralph "Fix the tests"` with Claude:

```
[ralph] Agent command: claude -p --output-format stream-json 'Fix the tests'
[claude] Read: internal/config/config.go
[claude] Read: internal/config/config_test.go
[claude] "I'll fix the failing test by updating the expected value..."
[claude] Edit: internal/config/config_test.go
[claude] Bash: go test ./internal/config/...
[claude] Complete (cost: $0.0150)
```

With `--verbose`:
```
[ralph] Agent command: claude -p --output-format stream-json 'Fix the tests'
[claude] Read: internal/config/config.go
[claude]   -> package config...
[claude] Edit: internal/config/config_test.go
[claude]   -> (edited 3 lines)
[claude] Bash: go test ./internal/config/...
[claude]   -> PASS ok pkg 0.5s
[claude] Complete (cost: $0.0150)
```

With tool error:
```
[claude] Edit: /etc/passwd
[claude] ERROR: Permission denied
```

---

## Architecture Notes

**Parser Interface Pattern:**
- Each agent has its own parser implementing `Parser` interface
- `ParserFor(agentCommand)` factory selects the right parser
- `OutputFlags(agentCommand)` returns agent-specific CLI flags
- Unknown agents return `nil` → fall back to raw output streaming
- Easy to add new agents: create `newagent.go` with parser, add case to factory

**Thread Safety:**
- `Formatter` uses mutex for safe concurrent writes
- `Processor` uses atomic operations for stats
- Goroutine-safe design for multi-writer scenarios

**Observability:**
- `Processor.LastActivity()` for stuck detection
- `Processor.Stats()` for event/error counts
- Optional debug logger for troubleshooting

**Graceful Degradation:**
- Unknown message types logged as `EventUnknown`, not errors
- Malformed JSON skipped, stream continues
- Parser errors don't crash processor

**Future Extensibility:**
- Different formatters (verbose, quiet, JSON pass-through)
- Event callbacks for programmatic integration
- Per-agent output flags in `OutputFlags()`
- Color themes / custom prefixes
