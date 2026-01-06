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
	_, _ = prefixColor.Fprint(f.out, prefix)
	_, _ = contentColor.Fprintln(f.out, content)
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
