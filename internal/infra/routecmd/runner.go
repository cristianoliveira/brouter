package routecmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"
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

	// The overflow kill is wired BEFORE Start with a nil-process
	// guard: the writer's Write can race the Start that fills
	// cmd.Process, so the process pointer crosses an atomic and the
	// callback no-ops until it is published. Without this, an
	// instantly-overflowing command would race the assignment.
	var proc atomic.Pointer[os.Process]
	out := newCappedWriter(OutputCap)
	errOut := newCappedWriter(OutputCap)
	out.kill = func() { killGroup(proc.Load()) }
	errOut.kill = out.kill
	cmd.Stdout = out
	cmd.Stderr = errOut

	if err := cmd.Start(); err != nil {
		return nil, ErrCommandUnavailable
	}
	proc.Store(cmd.Process)

	// Kill triggers: the deadline timer AND parent cancellation both
	// kill the WHOLE process group immediately — CommandContext alone
	// kills only the direct child, leaving descendants to hold the
	// pipes and stall the decision past the caller's patience.
	killOnce := sync.Once{}
	killNow := func() { killOnce.Do(func() { killGroup(cmd.Process) }) }
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

// groupAttr puts the child in its own process group so a timeout can
// kill descendants too, not just the direct child.
func groupAttr() *syscall.SysProcAttr {
	if runtime.GOOS == "windows" {
		return &syscall.SysProcAttr{}
	}
	return &syscall.SysProcAttr{Setpgid: true}
}

// killGroup kills the child's whole process group where supported.
func killGroup(p *os.Process) {
	if p == nil {
		return
	}
	if runtime.GOOS != "windows" {
		_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
		return
	}
	_ = p.Kill()
}
