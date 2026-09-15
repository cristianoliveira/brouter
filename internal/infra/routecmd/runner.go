package routecmd

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sync"
)

// execRunner is the production Runner: direct argv (no shell), the
// request on stdin, hard wall-clock kill, and bounded stdout/stderr
// capture. Stderr is captured for bounded disposal only — it never
// enters responses.
type execRunner struct{}

// Run implements Runner.
func (execRunner) Run(ctx context.Context, argv []string, stdin []byte) (stdout []byte, err error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = bytes.NewReader(stdin)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCommandError, err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCommandError, err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCommandError, err)
	}

	out := newCappedBuffer(OutputCap)
	errOut := newCappedBuffer(OutputCap)
	var wg sync.WaitGroup
	copyErrs := make(chan error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); copyErrs <- copyCapped(out, stdoutPipe) }()
	go func() { defer wg.Done(); copyErrs <- copyCapped(errOut, stderrPipe) }()
	wg.Wait()
	close(copyErrs)
	for copyErr := range copyErrs {
		if copyErr != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, copyErr
		}
	}

	waitErr := cmd.Wait()

	if ctx.Err() == context.DeadlineExceeded {
		return nil, ErrCommandTimeout
	}
	if waitErr != nil {
		return nil, ErrCommandError
	}
	if out.over {
		return nil, ErrCommandOutputCap
	}
	return out.buf.Bytes(), nil
}
