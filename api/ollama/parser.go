package ollama

import "encoding/json"

type Parser struct{}

func NewParser() *Parser { return &Parser{} }

func (p *Parser) Parse(data []byte) (*Response, error) {
	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
