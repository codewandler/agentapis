package completions

type StreamResult struct {
	Event        *Chunk
	Err          error
	RawEventName string
	RawJSON      []byte
}
