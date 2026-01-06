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
