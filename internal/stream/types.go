package stream

import "time"

// EventType categorizes parsed events
type EventType int

const (
	EventToolStart EventType = iota // Tool invocation began
	EventToolEnd                    // Tool completed (success or error)
	EventText                       // Assistant text output
	EventResult                     // Final result/completion
	EventProgress                   // Cost update, status change
	EventTodo                       // Task list update (TodoWrite)
	EventUnknown                    // Unrecognized message (for debugging)
)

func (t EventType) String() string {
	return [...]string{"ToolStart", "ToolEnd", "Text", "Result", "Progress", "Todo", "Unknown"}[t]
}

// TodoItem represents a task in a todo list
type TodoItem struct {
	ID       string // Task identifier
	Content  string // Task description
	Status   string // "pending", "in_progress", "completed"
	Priority string // "high", "medium", "low" (optional)
}

// Event represents a parsed agent output event (agent-agnostic)
type Event struct {
	Type      EventType
	Timestamp time.Time // When event was received

	// Tool events
	ToolName   string // e.g., "Read", "Edit", "Bash"
	ToolID     string // Correlation ID for start/end matching
	ToolInput  string // Summary of input (file path, command, etc.)
	ToolOutput string // Summary of output (for ToolEnd)
	ToolError  string // Error message if tool failed

	// Text events
	Text string // Assistant text output

	// Result events
	Result     string // Final result/completion text
	IsComplete bool   // Signals end of stream

	// Cost tracking
	Cost      float64 // Cumulative cost at this point
	CostDelta float64 // Cost of this specific operation

	// Todo events
	TodoItems []TodoItem // Task list items (for EventTodo)
}

// IsError returns true if this event represents a failure
func (e *Event) IsError() bool {
	return e.ToolError != ""
}
