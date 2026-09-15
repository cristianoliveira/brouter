package routecmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

// Run implements Runner.
//
// Kill discipline: the child runs in its own process group and the
// whole group is killed at the deadline, so descendants that inherited
// the pipes cannot outlive the decision. Wait runs concurrently with
// the output copies — waiting on the copies first could hang forever
// on a descendant-held pipe, because Go closes those pipes only inside
// Wait.
func (execRunner) Run(ctx context.Context, argv []string, stdin []byte) (stdout []byte, err error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.SysProcAttr = groupAttr()
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

	// Deadline kill: fires even while we are blocked on Wait/copies.
	killer := time.AfterFunc(Timeout, func() { killGroup(cmd.Process) })
	defer killer.Stop()

	out := newCappedBuffer(OutputCap)
	errOut := newCappedBuffer(OutputCap)
	var wg sync.WaitGroup
	copyErrs := make(chan error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); copyErrs <- copyCapped(out, stdoutPipe) }()
	go func() { defer wg.Done(); copyErrs <- copyCapped(errOut, stderrPipe) }()

	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	// Wait for process exit; Wait closes the pipes, which unblocks the
	// copies (post-kill read errors are expected noise).
	waitErr := <-waitCh
	wg.Wait()
	close(copyErrs)
	timedOut := ctx.Err() == context.DeadlineExceeded

	var copyErr error
	for e := range copyErrs {
		if e != nil && copyErr == nil {
			copyErr = e
		}
	}

	switch {
	case timedOut:
		return nil, ErrCommandTimeout
	case errors.Is(copyErr, ErrCommandOutputCap):
		return nil, ErrCommandOutputCap
	case copyErr != nil:
		// A genuine read failure with no timeout — surfaced, not hidden.
		return nil, fmt.Errorf("%w: reading command output failed", ErrCommandError)
	case waitErr != nil:
		return nil, ErrCommandError
	case out.over:
		return nil, ErrCommandOutputCap
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
