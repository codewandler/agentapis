// Package conversation provides a stateful conversation abstraction over unified streaming clients.
//
// Session history is canonical unified state and preserves assistant output ordering exactly as observed.
// Outbound replay messages are derived from that canonical state via a configurable MessageProjector.
// The default projector implements the library's standard replay/native continuation behavior, while
// custom projectors can apply service-specific replay shaping without mutating canonical session history.
// Session.Request exposes the smaller agent-facing event stream on top of the richer unified stream events, while RequestUnified remains the richer low-level escape hatch. 
package conversation
