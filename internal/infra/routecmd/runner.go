package routecmd

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"
)

// execRunner is the production Runner: direct argv (no shell), the
// request on stdin, hard wall-clock kill, and bounded stdout/stderr
// capture. Stderr is captured for bounded disposal only — it never
// enters responses.
type execRunner struct{}

// cappedWriter is a size-bounded io.Writer: once the limit is crossed
// it flags overflow and fails, which stops exec's internal copying.
type cappedWriter struct {
	limit int
	buf   bytes.Buffer
	over  bool
}

func newCappedWriter(limit int) *cappedWriter {
	return &cappedWriter{limit: limit}
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if w.buf.Len()+len(p) > w.limit {
		w.over = true
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

	out := newCappedWriter(OutputCap)
	errOut := newCappedWriter(OutputCap)
	cmd.Stdout = out
	cmd.Stderr = errOut

	if err := cmd.Start(); err != nil {
		return nil, ErrCommandUnavailable
	}

	// Deadline kill: fires even while Wait is blocked on descendants.
	killer := time.AfterFunc(Timeout, func() { killGroup(cmd.Process) })
	defer killer.Stop()

	waitErr := cmd.Wait()

	switch {
	case ctx.Err() == context.DeadlineExceeded:
		return nil, ErrCommandTimeout
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
	if runtime.GOOS != "windows" {
		_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
		return
	}
	_ = p.Kill()
}
