package stream

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/muesli/termenv"
)

// Visual indicators
const (
	iconToolStart = "⏺"
	iconSuccess   = "✅"
	iconError     = "❌"
	iconContinue  = "⎿"
	iconTodoDone  = "✅"
	iconTodoRun   = "🔄"
	iconTodoPend  = "⏸️"
	iconTodoList  = "📋"
	iconProgress  = "📊"
)

// FormatterConfig controls output behavior
type FormatterConfig struct {
	AgentName      string // Display name, e.g., "claude"
	ShowText       bool   // Show assistant text (false = tools only)
	ShowProgress   bool   // Show progress/unknown events
	UseColor       bool   // Enable ANSI colors
	UseEmoji       bool   // Use emoji indicators
	Verbose        bool   // Show extra details (tool output, durations)
	ShowTimestamp  bool   // Show [HH:MM:SS] prefix
	MaxOutputLines int    // Max lines to show in tool output (default 3)
	MaxOutputChars int    // Max chars per line (default 120)
}

// DefaultFormatterConfig returns sensible defaults
func DefaultFormatterConfig(agentName string) FormatterConfig {
	// Auto-detect terminal capabilities
	output := termenv.NewOutput(os.Stdout)
	useColor := output.ColorProfile() != termenv.Ascii

	return FormatterConfig{
		AgentName:      agentName,
		ShowText:       true,
		ShowProgress:   false,
		UseColor:       useColor,
		UseEmoji:       true,
		Verbose:        false,
		ShowTimestamp:  false,
		MaxOutputLines: 3,
		MaxOutputChars: 120,
	}
}

// PendingTool tracks a tool call waiting for its result
type PendingTool struct {
	Event     *Event
	Timestamp time.Time
}

// Formatter formats events for CLI display with tool correlation
type Formatter struct {
	out    *termenv.Output
	mu     sync.Mutex
	config FormatterConfig

	// Tool correlation state
	pendingTools map[string]*PendingTool // toolID -> pending call

	// Statistics tracking
	startTime  time.Time
	toolCount  int
	errorCount int
}

// NewFormatter creates a formatter with the given config
func NewFormatter(out io.Writer, config FormatterConfig) *Formatter {
	output := termenv.NewOutput(out)

	// Override color if disabled
	if !config.UseColor {
		output = termenv.NewOutput(out, termenv.WithProfile(termenv.Ascii))
	}

	// Apply defaults for unset values
	if config.MaxOutputLines == 0 {
		config.MaxOutputLines = 3
	}
	if config.MaxOutputChars == 0 {
		config.MaxOutputChars = 120
	}

	return &Formatter{
		out:          output,
		config:       config,
		pendingTools: make(map[string]*PendingTool),
		startTime:    time.Now(),
	}
}

// FormatEvent writes a formatted event to output with correlation
func (f *Formatter) FormatEvent(e *Event) {
	if e == nil {
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	switch e.Type {
	case EventToolStart:
		f.toolCount++
		// Store pending tool for correlation
		if e.ToolID != "" {
			f.pendingTools[e.ToolID] = &PendingTool{
				Event:     e,
				Timestamp: time.Now(),
			}
		}
		f.displayToolStart(e)

	case EventToolEnd:
		// Find matching start for correlation
		var pending *PendingTool
		var duration time.Duration
		if e.ToolID != "" {
			if p, ok := f.pendingTools[e.ToolID]; ok {
				pending = p
				duration = time.Since(p.Timestamp)
				delete(f.pendingTools, e.ToolID)
			}
		}
		if e.IsError() {
			f.errorCount++
		}
		f.displayToolResult(pending, e, duration)

	case EventText:
		if f.config.ShowText && e.Text != "" {
			f.displayText(e.Text)
		}

	case EventResult:
		f.displayCompletion(e)

	case EventTodo:
		f.displayTodo(e.TodoItems)

	case EventProgress, EventUnknown:
		if f.config.ShowProgress {
			f.displayProgress(e.Text)
		}
	}
}

// displayToolStart shows a tool invocation
func (f *Formatter) displayToolStart(e *Event) {
	var sb strings.Builder

	// Timestamp prefix if enabled
	if f.config.ShowTimestamp {
		sb.WriteString(f.dim(fmt.Sprintf("[%s] ", time.Now().Format("15:04:05"))))
	}

	// Icon
	if f.config.UseEmoji {
		sb.WriteString(f.cyan(iconToolStart + " "))
	} else {
		sb.WriteString(f.cyan("> "))
	}

	// Tool name and primary arg
	sb.WriteString(f.yellow(e.ToolName))
	if e.ToolInput != "" {
		sb.WriteString(f.dim("("))
		sb.WriteString(f.white(truncateInput(e.ToolInput, 80)))
		sb.WriteString(f.dim(")"))
	}
	sb.WriteString("\n")

	fmt.Fprint(f.out, sb.String())
}

// displayToolResult shows a tool result, optionally correlated with its start
func (f *Formatter) displayToolResult(pending *PendingTool, e *Event, duration time.Duration) {
	var sb strings.Builder

	// Determine icon and style based on success/error
	icon := iconSuccess
	isError := e.IsError()
	if isError {
		icon = iconError
	}

	// Output content
	output := e.ToolOutput
	if isError {
		output = e.ToolError
	}

	// Skip empty successful results in non-verbose mode
	if !isError && output == "" && !f.config.Verbose {
		return
	}

	// Timestamp prefix
	if f.config.ShowTimestamp {
		sb.WriteString(f.dim(fmt.Sprintf("[%s] ", time.Now().Format("15:04:05"))))
	}

	// Icon
	if f.config.UseEmoji {
		if isError {
			sb.WriteString(f.red(icon + " "))
		} else {
			sb.WriteString(f.green(icon + " "))
		}
	} else {
		if isError {
			sb.WriteString(f.red("[ERR] "))
		} else {
			sb.WriteString(f.green("[OK] "))
		}
	}

	// Result header with tool name correlation
	lines, chars := countLinesChars(output)
	if isError {
		sb.WriteString(f.red("Error"))
	} else {
		sb.WriteString(f.green("Result"))
	}

	// Show correlated tool name
	if pending != nil && pending.Event != nil && pending.Event.ToolName != "" {
		sb.WriteString(f.dim(" ← " + pending.Event.ToolName))
	}

	if output != "" {
		sb.WriteString(f.dim(fmt.Sprintf(" (%d lines, %d chars)", lines, chars)))
	}

	// Duration in verbose mode
	if f.config.Verbose && duration > 0 {
		sb.WriteString(f.dim(fmt.Sprintf(" [%s]", duration.Round(time.Millisecond))))
	}
	sb.WriteString("\n")

	// Output content with continuation lines
	if output != "" {
		f.appendTruncatedOutput(&sb, output, isError)
	}

	fmt.Fprint(f.out, sb.String())
}

// displayText shows assistant text output
func (f *Formatter) displayText(text string) {
	var sb strings.Builder

	for _, line := range strings.Split(text, "\n") {
		if f.config.ShowTimestamp {
			sb.WriteString(f.dim(fmt.Sprintf("[%s] ", time.Now().Format("15:04:05"))))
		}
		sb.WriteString(f.white(line))
		sb.WriteString("\n")
	}

	fmt.Fprint(f.out, sb.String())
}

// displayCompletion shows the final completion with statistics
func (f *Formatter) displayCompletion(e *Event) {
	var sb strings.Builder
	elapsed := time.Since(f.startTime)

	if f.config.ShowTimestamp {
		sb.WriteString(f.dim(fmt.Sprintf("[%s] ", time.Now().Format("15:04:05"))))
	}

	if f.config.UseEmoji {
		sb.WriteString(f.green(iconSuccess + " "))
	} else {
		sb.WriteString(f.green("[OK] "))
	}

	sb.WriteString(f.green("Complete"))

	// Build stats string
	var stats []string

	// Cost (always shown if non-zero)
	if e.Cost > 0 {
		stats = append(stats, fmt.Sprintf("cost: $%.2f", e.Cost))
	}

	// Token counts (shown if we have any)
	if e.InputTokens > 0 || e.OutputTokens > 0 {
		tokenStr := "tokens: " + formatTokenCount(e.InputTokens) + " in"
		if e.CacheReadTokens > 0 {
			tokenStr += " (" + formatTokenCount(e.CacheReadTokens) + " cached)"
		}
		tokenStr += " / " + formatTokenCount(e.OutputTokens) + " out"
		stats = append(stats, tokenStr)
	}

	// Tool and error counts
	stats = append(stats, fmt.Sprintf("tools: %d", f.toolCount))
	stats = append(stats, fmt.Sprintf("errors: %d", f.errorCount))

	// Elapsed time
	stats = append(stats, fmt.Sprintf("time: %s", formatDuration(elapsed)))

	sb.WriteString(f.dim(" (" + strings.Join(stats, ", ") + ")"))
	sb.WriteString("\n")

	fmt.Fprint(f.out, sb.String())
}

// formatTokenCount formats token counts with K/M suffixes
func formatTokenCount(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.0fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// formatDuration formats duration in a human-readable way
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if s == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dm%ds", m, s)
}

// displayTodo shows a task list update
func (f *Formatter) displayTodo(items []TodoItem) {
	if len(items) == 0 {
		return
	}

	var sb strings.Builder

	// Header
	if f.config.UseEmoji {
		sb.WriteString(f.cyan(iconTodoList + " "))
	}
	sb.WriteString(f.cyan("Todo List"))
	sb.WriteString("\n")

	// Items
	var completed, inProgress, pending int
	for _, item := range items {
		icon := iconTodoPend
		style := f.dim
		switch item.Status {
		case "completed":
			icon = iconTodoDone
			style = f.green
			completed++
		case "in_progress":
			icon = iconTodoRun
			style = f.yellow
			inProgress++
		default:
			pending++
		}

		sb.WriteString("  ")
		if f.config.UseEmoji {
			sb.WriteString(icon + " ")
		} else {
			// Fallback without emoji
			switch item.Status {
			case "completed":
				sb.WriteString("[x] ")
			case "in_progress":
				sb.WriteString("[>] ")
			default:
				sb.WriteString("[ ] ")
			}
		}
		sb.WriteString(style(truncateInput(item.Content, 70)))

		// Priority indicator
		if item.Priority != "" && item.Priority != "medium" {
			sb.WriteString(f.dim(fmt.Sprintf(" [%s]", item.Priority)))
		}

		// Active marker
		if item.Status == "in_progress" {
			sb.WriteString(f.yellow(" ← ACTIVE"))
		}
		sb.WriteString("\n")
	}

	// Progress summary with all counters
	total := len(items)
	pct := float64(completed) / float64(total) * 100
	if f.config.UseEmoji {
		sb.WriteString(f.dim(fmt.Sprintf("\n  %s Progress: %d/%d (%.0f%%) | %d active, %d pending\n",
			iconProgress, completed, total, pct, inProgress, pending)))
	} else {
		sb.WriteString(f.dim(fmt.Sprintf("\n  Progress: %d/%d (%.0f%%) | %d active, %d pending\n",
			completed, total, pct, inProgress, pending)))
	}

	fmt.Fprint(f.out, sb.String())
}

// displayProgress shows progress/status messages
func (f *Formatter) displayProgress(text string) {
	var sb strings.Builder

	if f.config.ShowTimestamp {
		sb.WriteString(f.dim(fmt.Sprintf("[%s] ", time.Now().Format("15:04:05"))))
	}
	sb.WriteString(f.dim(text))
	sb.WriteString("\n")

	fmt.Fprint(f.out, sb.String())
}

// appendTruncatedOutput adds truncated output with continuation markers
func (f *Formatter) appendTruncatedOutput(sb *strings.Builder, output string, isError bool) {
	lines := strings.Split(output, "\n")
	maxLines := f.config.MaxOutputLines
	maxChars := f.config.MaxOutputChars

	style := f.dim
	if isError {
		style = f.red
	}

	// First line with continuation marker
	if len(lines) > 0 && lines[0] != "" {
		first := truncateInput(lines[0], maxChars)
		sb.WriteString("  ")
		if f.config.UseEmoji {
			sb.WriteString(f.dim(iconContinue + "  "))
		} else {
			sb.WriteString(f.dim("|  "))
		}
		sb.WriteString(style(first))
		sb.WriteString("\n")
	}

	// Additional lines (indented more, dimmed)
	shown := 1
	for i := 1; i < len(lines) && shown < maxLines; i++ {
		if lines[i] == "" {
			continue
		}
		line := truncateInput(lines[i], maxChars)
		sb.WriteString("      ")
		sb.WriteString(f.dim(line))
		sb.WriteString("\n")
		shown++
	}

	// Show "more lines" indicator if truncated
	remaining := len(lines) - maxLines
	if remaining > 0 {
		sb.WriteString("  ")
		if f.config.UseEmoji {
			sb.WriteString(f.dim(iconContinue + "  "))
		} else {
			sb.WriteString(f.dim("|  "))
		}
		sb.WriteString(f.dim(fmt.Sprintf("... %d more lines", remaining)))
		sb.WriteString("\n")
	}
}

// Style helpers using termenv
func (f *Formatter) cyan(s string) string {
	return f.out.String(s).Foreground(f.out.Color("6")).String()
}

func (f *Formatter) yellow(s string) string {
	return f.out.String(s).Foreground(f.out.Color("3")).String()
}

func (f *Formatter) green(s string) string {
	return f.out.String(s).Foreground(f.out.Color("2")).String()
}

func (f *Formatter) red(s string) string {
	return f.out.String(s).Foreground(f.out.Color("1")).Bold().String()
}

func (f *Formatter) white(s string) string {
	return f.out.String(s).Foreground(f.out.Color("7")).String()
}

func (f *Formatter) dim(s string) string {
	return f.out.String(s).Faint().String()
}

// Helper functions

func countLinesChars(s string) (lines, chars int) {
	if s == "" {
		return 0, 0
	}
	chars = len(s)
	lines = strings.Count(s, "\n") + 1
	return
}

func truncateInput(s string, max int) string {
	// First, get only first line if multiline
	if idx := strings.Index(s, "\n"); idx != -1 {
		s = s[:idx]
	}
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
