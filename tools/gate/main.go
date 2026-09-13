// Command gate implements the single normal quality gate. Success prints
// exactly "true"; any failing step prints "false", bounded diagnostics, and
// a pointer to the full local log, then exits nonzero. Formatting, static
// analysis, race, coverage, security, and architecture checks are separate
// commands and must not be added here.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// boundedLines caps diagnostics on stdout; the full output always goes to
// the log file.
const boundedLines = 20

type step struct {
	name    string
	command string
}

// runCheck executes steps in the pinned order, failing fast. Tool output is
// captured to the log; stdout carries only the contract output. The shell is
// injected so tests stay independent of PATH contents.
func runCheck(shell string, steps []step, logPath string, stdout io.Writer) int {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		fmt.Fprintf(stdout, "false\ncannot create log directory: %v\n", err)
		return 1
	}
	logFile, err := os.Create(logPath)
	if err != nil {
		fmt.Fprintf(stdout, "false\ncannot write log %s: %v\n", logPath, err)
		return 1
	}
	defer logFile.Close()

	for _, s := range steps {
		fmt.Fprintf(logFile, "=== %s\n$ %s\n", s.name, s.command)

		cmd := exec.Command(shell, "-c", s.command)
		cmd.Env = pinnedEnv()
		out, err := cmd.CombinedOutput()
		logFile.Write(out)

		if err != nil {
			fmt.Fprintf(stdout, "false\nstep %q failed: %v\nfirst %d line(s) of output:\n", s.name, exitMessage(err), boundedLines)
			writeBounded(stdout, out, boundedLines)
			fmt.Fprintf(stdout, "full log: %s\n", logPath)
			return 1
		}
	}

	fmt.Fprintln(stdout, "true")
	return 0
}

// pinnedEnv runs steps with explicit, version-stable tool behavior:
// GOTOOLCHAIN=local forbids silent toolchain downloads, and a fixed locale
// keeps tool output ordering stable.
func pinnedEnv() []string {
	env := []string{"GOTOOLCHAIN=local", "LC_ALL=C"}
	for _, kv := range os.Environ() {
		switch {
		case strings.HasPrefix(kv, "GOTOOLCHAIN="), strings.HasPrefix(kv, "LC_ALL="):
		default:
			env = append(env, kv)
		}
	}
	return env
}

func exitMessage(err error) string {
	var exitErr *exec.ExitError
	if e, ok := err.(*exec.ExitError); ok {
		exitErr = e
		return fmt.Sprintf("exit status %d", exitErr.ExitCode())
	}
	return err.Error()
}

func writeBounded(w io.Writer, out []byte, limit int) {
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) > limit {
		lines = lines[:limit]
	}
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
}

func main() {
	shell, err := exec.LookPath("sh")
	if err != nil {
		fmt.Fprintln(os.Stdout, "false")
		fmt.Fprintf(os.Stdout, "sh: not found in PATH. A POSIX shell is required; on NixOS use the dev shell (nix develop).\n")
		os.Exit(1)
	}

	steps := []step{
		{name: "lint", command: "go run ./tools/lint ."},
		{name: "build", command: "go build ./..."},
		{name: "test", command: "go test -vet=off ./..."},
	}
	os.Exit(runCheck(shell, steps, filepath.Join(".tmp", "check.log"), os.Stdout))
}
