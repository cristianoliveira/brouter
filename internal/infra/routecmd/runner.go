package routecmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// execRunner is the production Runner: direct argv (no shell), the
// request on stdin, hard wall-clock kill, and bounded stdout/stderr
// capture. Stderr is captured for bounded disposal only — it never
// enters responses.
type execRunner struct{}

// cappedWriter is a size-bounded io.Writer: crossing the limit flags
// overflow and IMMEDIATELY kills the child's process group (a command
// that overruns must not keep running until the timeout), then fails
// the write so exec's internal copying stops.
type cappedWriter struct {
	limit int
	buf   bytes.Buffer
	over  bool
	kill  func() // wired after Start, once the process exists
}

func newCappedWriter(limit int) *cappedWriter {
	return &cappedWriter{limit: limit}
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if w.buf.Len()+len(p) > w.limit {
		w.over = true
		if w.kill != nil {
			w.kill()
		}
		return 0, errOutputCap
	}
	w.buf.Write(p)
	return len(p), nil
}

// Run implements Runner.
//
// Kill discipline: the child runs in its own process group and the
// whole group is killed at the deadline, so descendants that inherited
// stdout/stderr cannot outlive the decision or keep exec's internal
// copy goroutines alive past it.
func (execRunner) Run(ctx context.Context, argv []string, stdin []byte) (stdout []byte, err error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.SysProcAttr = groupAttr()
	cmd.Stdin = bytes.NewReader(stdin)

	// The overflow kill is wired BEFORE Start through a kill gate: a
	// kill fired before publication is recorded as pending and
	// executes the instant the process is published — an
	// instantly-overflowing child is terminated immediately and
	// deterministically, never racing publication.
	gate := &killGate{}
	out := newCappedWriter(OutputCap)
	errOut := newCappedWriter(OutputCap)
	out.kill = gate.kill
	errOut.kill = gate.kill
	cmd.Stdout = out
	cmd.Stderr = errOut

	if err := cmd.Start(); err != nil {
		// A pre-canceled or expired context fails in Start; classify
		// that as the caller's cancellation, not an unavailable
		// command. Any pending kill is simply dropped with the failed
		// start — nothing is left to hang.
		return nil, startFailure(err)
	}
	gate.publish(cmd.Process)

	// Kill triggers: the deadline timer AND parent cancellation both
	// kill the WHOLE process group immediately — CommandContext alone
	// kills only the direct child, leaving descendants to hold the
	// pipes and stall the decision past the caller's patience.
	killOnce := sync.Once{}
	killNow := func() { killOnce.Do(func() { gate.kill() }) }
	killer := time.AfterFunc(Timeout, killNow)
	defer killer.Stop()
	ctxWatch := make(chan struct{})
	defer close(ctxWatch)
	go func() {
		select {
		case <-ctx.Done():
			killNow()
		case <-ctxWatch:
		}
	}()

	waitErr := cmd.Wait()

	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return nil, ErrCommandTimeout
	case errors.Is(ctx.Err(), context.Canceled):
		return nil, ErrCommandCanceled
	case out.over || errOut.over:
		return nil, ErrCommandOutputCap
	case waitErr != nil:
		return nil, ErrCommandError
	}
	return out.buf.Bytes(), nil
}

// startFailure classifies a failed spawn: a pre-canceled or expired
// context is the caller's cancellation, not an unavailable command.
func startFailure(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return ErrCommandCanceled
	case errors.Is(err, context.DeadlineExceeded):
		return ErrCommandTimeout
	default:
		return ErrCommandUnavailable
	}
}

// groupAttr puts the child in its own process group so a timeout can
// kill descendants too, not just the direct child.
func groupAttr() *syscall.SysProcAttr {
	if runtime.GOOS == "windows" {
		return &syscall.SysProcAttr{}
	}
	return &syscall.SysProcAttr{Setpgid: true}
}

// killProcess kills the child's whole process group where supported.
func killProcess(p *os.Process) {
	if p == nil {
		return
	}
	if runtime.GOOS != "windows" {
		_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
		return
	}
	_ = p.Kill()
}
