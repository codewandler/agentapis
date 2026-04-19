package ollama

type StreamResult struct {
	Event        *Response
	Err          error
	RawEventName string
	RawJSON      []byte
}
