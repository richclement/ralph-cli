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
	rawLog     io.Writer

	// Observability
	lastActivity atomic.Value // time.Time
	eventCount   atomic.Int64
	errorCount   atomic.Int64
	debugLog     *log.Logger // Optional, for verbose debugging
}

// NewProcessor creates a processor for the given agent command
// Returns nil if agent doesn't support structured output parsing
func NewProcessor(agentCommand string, formatter *Formatter, debugLog *log.Logger, rawLog io.Writer) *Processor {
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
		rawLog:     rawLog,
	}
	p.lastActivity.Store(time.Now())
	go p.decodeLoop()
	return p
}

func (p *Processor) decodeLoop() {
	defer close(p.done)

	// Read newline-delimited JSON without line length limits.
	reader := bufio.NewReader(p.pipeReader)

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			trimmed := bytes.TrimSpace(line)
			if len(trimmed) > 0 {
				if p.rawLog != nil {
					if _, logErr := p.rawLog.Write(append(trimmed, '\n')); logErr != nil {
						p.errorCount.Add(1)
						if p.debugLog != nil {
							p.debugLog.Printf("raw log write error: %v", logErr)
						}
					}
				}
				p.lastActivity.Store(time.Now())

				// Validate JSON before parsing
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
			if err != io.EOF {
				p.errorCount.Add(1)
				if p.debugLog != nil {
					p.debugLog.Printf("reader error: %v", err)
				}
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
		_ = p.pipeWriter.Close()
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
