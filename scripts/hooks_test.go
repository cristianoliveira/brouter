// Package scripts hosts the controlled tests for the quality gate and its
// enforcement points (hooks). No program logic lives here by design.
package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initRepo creates a temporary git repository with the shared hook
// directory copied in, mirroring a fresh clone before installation.
func initRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	run := exec.Command("git", "init", "-q")
	run.Dir = repo
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	return repo
}

// copyHookDir copies the repository's versioned .githooks into a temp
// repo so tests exercise the exact scripts that ship.
func copyHookDir(t *testing.T, repo string) {
	t.Helper()

	src := filepath.Join("..", ".githooks")
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(repo, ".githooks")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(src, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, entry.Name()), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func mustAbs(rel string) string {
	abs, err := filepath.Abs(rel)
	if err != nil {
		return rel
	}
	return abs
}

// runIn executes an absolute script path with dir as working directory.
func runIn(dir, script string, args ...string) (string, error) {
	cmd := exec.Command("/bin/sh", append([]string{script}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func gitIn(t *testing.T, repo string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func TestHooksInstallOnCleanRepoSetsHooksPath(t *testing.T) {
	// Given a fresh clone without hooks configured, when hooks install,
	// core.hooksPath points at the versioned directory and check passes.
	repo := initRepo(t)
	copyHookDir(t, repo)

	if out, err := runIn(repo, mustAbs("../scripts/hooks-install.sh")); err != nil {
		t.Fatalf("install failed: %v: %s", err, out)
	}
	if got := strings.TrimSpace(gitIn(t, repo, "config", "core.hooksPath")); got != ".githooks" {
		t.Errorf("core.hooksPath = %q, want .githooks", got)
	}

	checkOut, checkErr := runIn(repo, mustAbs("../scripts/hooks-check.sh"))
	if checkErr != nil {
		t.Fatalf("check failed: %v: %s", checkErr, checkOut)
	}
	if checkOut != "true\n" {
		t.Errorf("check stdout = %q, want exactly %q", checkOut, "true\n")
	}
}

func TestHooksInstallIsIdempotent(t *testing.T) {
	// Given hooks are already installed, when install runs again, it
	// succeeds without changing anything.
	repo := initRepo(t)
	copyHookDir(t, repo)
	if out, err := runIn(repo, mustAbs("../scripts/hooks-install.sh")); err != nil {
		t.Fatalf("first install failed: %v: %s", err, out)
	}

	if out, err := runIn(repo, mustAbs("../scripts/hooks-install.sh")); err != nil {
		t.Fatalf("second install failed: %v: %s", err, out)
	}
	if got := strings.TrimSpace(gitIn(t, repo, "config", "core.hooksPath")); got != ".githooks" {
		t.Errorf("core.hooksPath = %q, want .githooks", got)
	}
}

func TestHooksInstallRefusesForeignHooksPath(t *testing.T) {
	// Given core.hooksPath is managed by something else, when hooks
	// install, it refuses with actionable guidance instead of overwriting.
	repo := initRepo(t)
	copyHookDir(t, repo)
	gitIn(t, repo, "config", "core.hooksPath", "my-own-hooks")

	out, err := runIn(repo, mustAbs("../scripts/hooks-install.sh"))

	if err == nil {
		t.Fatal("install succeeded over foreign hooksPath, want refusal")
	}
	if !strings.Contains(out, "my-own-hooks") {
		t.Errorf("output = %q, want the existing path named", out)
	}
	if !strings.Contains(out, "make hooks-install") {
		t.Errorf("output = %q, want recovery guidance", out)
	}
	if got := strings.TrimSpace(gitIn(t, repo, "config", "core.hooksPath")); got != "my-own-hooks" {
		t.Errorf("core.hooksPath changed to %q, want untouched", got)
	}
}

func TestHooksInstallRefusesActiveLegacyHooks(t *testing.T) {
	// Given .git/hooks contains an active non-sample hook, when hooks
	// install, it refuses and names the file so nothing is silenced.
	repo := initRepo(t)
	copyHookDir(t, repo)
	legacy := filepath.Join(repo, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(legacy, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	out, err := runIn(repo, mustAbs("../scripts/hooks-install.sh"))

	if err == nil {
		t.Fatal("install succeeded over active legacy hooks, want refusal")
	}
	if !strings.Contains(out, "pre-commit") {
		t.Errorf("output = %q, want the active hook named", out)
	}
	if _, serr := os.Stat(legacy); serr != nil {
		t.Error("legacy hook was modified")
	}
}

func TestHooksCheckFailsWithGuidanceWhenNotInstalled(t *testing.T) {
	// Given hooks were never installed, when check runs, it prints false
	// and points at the install command.
	repo := initRepo(t)

	out, err := runIn(repo, mustAbs("../scripts/hooks-check.sh"))

	if err == nil {
		t.Fatal("check succeeded without install, want failure")
	}
	if !strings.HasPrefix(out, "false\n") {
		t.Errorf("output = %q, want prefix %q", out, "false\n")
	}
	if !strings.Contains(out, "make hooks-install") {
		t.Errorf("output = %q, want install guidance", out)
	}
}

func TestCommitMsgAcceptsConventionalTaskCommits(t *testing.T) {
	// Given conventional subjects referencing a task ID, when the hook
	// runs, they pass.
	valid := []string{
		"feat(guardrails): TASK-0004 add versioned hooks",
		"fix: TASK-0002 correct usage output",
		"plans(edit): TASK-0001 confirm support contract",
		"docs: TASK-0009 explain routing semantics",
	}
	for _, subject := range valid {
		t.Run(subject, func(t *testing.T) {
			msg := filepath.Join(t.TempDir(), "msg")
			if err := os.WriteFile(msg, []byte(subject+"\n\nbody\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			if out, err := runIn(t.TempDir(), mustAbs("../.githooks/commit-msg"), msg); err != nil {
				t.Errorf("hook rejected %q: %v: %s", subject, err, out)
			}
		})
	}
}

func TestCommitMsgRejectsNonConventionalSubjects(t *testing.T) {
	// Given subjects that break the convention or omit the task ID, when
	// the hook runs, it rejects them with bounded, actionable errors.
	rejected := []struct {
		name, subject, want string
	}{
		{name: "plain prose is not conventional", subject: "updated some files", want: "conventional"},
		{name: "unknown type is rejected", subject: "magic: TASK-0004 do things", want: "conventional"},
		{name: "missing task reference is rejected", subject: "feat: add hooks without an id", want: "TASK-"},
	}
	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			msg := filepath.Join(t.TempDir(), "msg")
			if err := os.WriteFile(msg, []byte(tt.subject+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			out, err := runIn(t.TempDir(), mustAbs("../.githooks/commit-msg"), msg)
			if err == nil {
				t.Fatalf("hook accepted %q", tt.subject)
			}
			if !strings.Contains(out, tt.want) {
				t.Errorf("output = %q, want %q", out, tt.want)
			}
		})
	}
}

func TestCommitMsgExemptsMergeCommits(t *testing.T) {
	// Given a merge commit subject generated by git, when the hook runs,
	// it passes without conventional formatting.
	msg := filepath.Join(t.TempDir(), "msg")
	if err := os.WriteFile(msg, []byte("Merge pull request #4 from cristianoliveira/task\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if out, err := runIn(t.TempDir(), mustAbs("../.githooks/commit-msg"), msg); err != nil {
		t.Errorf("hook rejected merge commit: %v: %s", err, out)
	}
}

func TestPreCommitInvokesExactlyMakeCheck(t *testing.T) {
	// Given the hook runs, it invokes exactly `make check` — nothing else,
	// proving hooks and CI/watcher share the same gate command.
	repo := initRepo(t)
	copyHookDir(t, repo)

	calls := filepath.Join(t.TempDir(), "calls")
	fakeDir := t.TempDir()
	fake := "#!/bin/sh\nprintf '%s\\n' \"$@\" >> " + calls + "\nexit 0\n"
	if err := os.WriteFile(filepath.Join(fakeDir, "make"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("/bin/sh", mustAbs("../.githooks/pre-commit"))
	cmd.Dir = repo
	cmd.Env = []string{"PATH=" + fakeDir + string(os.PathListSeparator) + os.Getenv("PATH")}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pre-commit failed: %v: %s", err, out)
	}

	data, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal("make was never invoked")
	}
	if strings.TrimSpace(string(data)) != "check" {
		t.Errorf("make invoked with %q, want exactly \"check\"", string(data))
	}
}
