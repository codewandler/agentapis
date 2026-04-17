package completions

import "encoding/json"

type Parser struct{}

func NewParser() *Parser { return &Parser{} }

func (p *Parser) Parse(_ string, data []byte) (*Chunk, error) {
	if string(data) == StreamDone {
		return nil, nil
	}
	var chunk Chunk
	if err := json.Unmarshal(data, &chunk); err != nil {
		return nil, err
	}
	return &chunk, nil
}
