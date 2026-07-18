package socketio

import (
	"errors"
	"strings"
)

// reservedEvents are the event names Socket.IO reserves for lifecycle
// signalling. Applications may not emit them as ordinary events; the server and
// client emit them internally.
var reservedEvents = map[string]struct{}{
	"connect":        {},
	"connect_error":  {},
	"disconnect":     {},
	"disconnecting":  {},
	"newListener":    {},
	"removeListener": {},
}

// IsReservedEvent reports whether name is a Socket.IO reserved event name
// (such as "connect", "disconnect", or "connect_error"). Reserved names are
// emitted by the library itself and must not be used for application events,
// mirroring the RESERVED_EVENTS guard in the JavaScript server.
func IsReservedEvent(name string) bool {
	_, ok := reservedEvents[name]
	return ok
}

// ErrReservedEvent indicates an attempt to use a reserved event name for an
// application event.
var ErrReservedEvent = errors.New("socketio: reserved event name")

// ErrEmptyEvent indicates an empty event name.
var ErrEmptyEvent = errors.New("socketio: empty event name")

// ValidateEventName checks that name is usable as an application event name: it
// must be non-empty and must not collide with a reserved Socket.IO event. It
// returns ErrEmptyEvent or ErrReservedEvent on failure, and nil when the name is
// safe to emit.
func ValidateEventName(name string) error {
	if name == "" {
		return ErrEmptyEvent
	}
	if IsReservedEvent(name) {
		return ErrReservedEvent
	}
	return nil
}

// ReservedEvents returns a sorted copy of the reserved event names, useful for
// documentation and tooling.
func ReservedEvents() []string {
	out := make([]string, 0, len(reservedEvents))
	for k := range reservedEvents {
		out = append(out, k)
	}
	// insertion sort keeps this dependency-free and the list is tiny.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && strings.Compare(out[j-1], out[j]) > 0; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
