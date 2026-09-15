package routecmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

// cappedBuffer accumulates at most limit bytes and flags overflow.
type cappedBuffer struct {
	buf   bytes.Buffer
	limit int
	over  bool
}

func newCappedBuffer(limit int) *cappedBuffer {
	return &cappedBuffer{limit: limit}
}

// copyCapped copies r into b, killing the copy as soon as the cap is
// exceeded. The parent then kills the child (runner handles it).
func copyCapped(b *cappedBuffer, r io.Reader) error {
	chunk := make([]byte, 512)
	for {
		if b.buf.Len() > b.limit {
			b.over = true
			return ErrCommandOutputCap
		}
		n, err := r.Read(chunk)
		if n > 0 {
			b.buf.Write(chunk[:n])
			if b.buf.Len() > b.limit {
				b.over = true
				return ErrCommandOutputCap
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("route_command: read output: %w", err)
		}
	}
}
