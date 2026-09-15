package routecmd

import "errors"

// errOutputCap aborts exec's internal copying on overflow. The
// authoritative signal is the writer's over flag, checked after Wait:
// exec discards copy-goroutine errors, and a deadline kill produces
// the same abort, so the flag keeps classification unambiguous.
var errOutputCap = errors.New("output cap exceeded")
