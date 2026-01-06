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
