package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The NixOS desktop handler is a generated wrapper plus a desktop
// entry: the wrapper forwards URL arguments to the brouter binary in
// the same package with an absolute config path; the entry declares
// the http and https schemes. The Nix expression substitutes the same
// templates these tests exercise, so the contract is locked once.

func renderTemplate(t *testing.T, template, placeholder, value string) string {
	t.Helper()
	data, err := os.ReadFile(template)
	if err != nil {
		t.Fatalf("template %s missing: %v", template, err)
	}
	return strings.ReplaceAll(string(data), placeholder, value)
}

func writeRecordingStub(t *testing.T, dir string, exitStatus int) string {
	t.Helper()
	stub := filepath.Join(dir, "stub-brouter")
	stubLog := filepath.Join(dir, "stub-argv.log")
	script := "#!/bin/sh\n" +
		"for arg in \"$@\"; do printf '%s\\n' \"$arg\" >> " + stubLog + "; done\n" +
		"printf 'brouter diagnostic on stderr\\n' >&2\n" +
		"exit " + itoa(exitStatus) + "\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return stub
}

func itoa(n int) string {
	digits := "0123456789"
	if n == 0 {
		return "0"
	}
	out := ""
	for n > 0 {
		out = string(digits[n%10]) + out
		n /= 10
	}
	return out
}

func runWrapper(t *testing.T, wrapper string, env []string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(wrapper, args...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	status := 0
	if exit, ok := err.(*exec.ExitError); ok {
		status = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("wrapper run failed: %v\n%s", err, out)
	}
	return string(out), status
}

func TestNixosHandlerWrapperForwardsURLsToEmbeddedBrouter(t *testing.T) {
	tmp := t.TempDir()
	stub := writeRecordingStub(t, tmp, 0)
	wrapper := filepath.Join(tmp, "brouter-handler")
	if err := os.WriteFile(wrapper, []byte(renderTemplate(t,
		"nixos/brouter-handler-wrapper.sh.in", "@brouter@", stub)), 0o755); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	rawURL := "https://company.example/page?q=14&tag=nixos#top"
	env := append(os.Environ(), "HOME="+home, "XDG_STATE_HOME="+filepath.Join(home, "state"))
	out, status := runWrapper(t, wrapper, env, rawURL)
	if status != 0 {
		t.Fatalf("wrapper status = %d, want 0\n%s", status, out)
	}

	// The wrapper forwards as structured arguments: [open, --config,
	// <abs user config>, <url>] — never a shell.
	argv := readArgvFile(t, filepath.Join(tmp, "stub-argv.log"))
	wantConfig := filepath.Join(home, ".config", "brouter", "config.toml")
	want := []string{"open", "--config", wantConfig, rawURL}
	if len(argv) != len(want) {
		t.Fatalf("stub argv = %q, want %q", argv, want)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Errorf("argv[%d] = %q, want %q", i, argv[i], want[i])
		}
	}

	// Diagnostics are not lost outside a terminal: the stub's stderr
	// lands in the handler state log.
	log := readFileOrFatal(t, filepath.Join(home, "state", "brouter", "handler.log"))
	if !strings.Contains(log, "brouter diagnostic on stderr") {
		t.Errorf("handler log = %q, want the child's diagnostics", log)
	}
}

func TestNixosHandlerWrapperResolvesConfigLikeTheCLI(t *testing.T) {
	tmp := t.TempDir()
	stub := writeRecordingStub(t, tmp, 0)
	wrapper := filepath.Join(tmp, "brouter-handler")
	if err := os.WriteFile(wrapper, []byte(renderTemplate(t,
		"nixos/brouter-handler-wrapper.sh.in", "@brouter@", stub)), 0o755); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	xdgConfig := filepath.Join(home, "xdg-config")
	env := append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+xdgConfig,
		"XDG_STATE_HOME="+filepath.Join(home, "state"))
	runWrapper(t, wrapper, env, "https://config.example/")

	argv := readArgvFile(t, filepath.Join(tmp, "stub-argv.log"))
	// os.UserConfigDir semantics: XDG_CONFIG_HOME wins over $HOME/.config,
	// exactly like the brouter CLI's default config resolution.
	wantConfig := filepath.Join(xdgConfig, "brouter", "config.toml")
	if len(argv) != 4 || argv[2] != wantConfig {
		t.Errorf("stub argv = %q, want config arg %q", argv, wantConfig)
	}
}

func TestNixosHandlerWrapperSurfacesFailureWithoutTerminal(t *testing.T) {
	tmp := t.TempDir()
	stub := writeRecordingStub(t, tmp, 3)
	wrapper := filepath.Join(tmp, "brouter-handler")
	if err := os.WriteFile(wrapper, []byte(renderTemplate(t,
		"nixos/brouter-handler-wrapper.sh.in", "@brouter@", stub)), 0o755); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	env := append(os.Environ(), "HOME="+home,
		"XDG_STATE_HOME="+filepath.Join(home, "state"),
		// No session bus in the environment: the bounded notification
		// must degrade silently and the failure must still surface.
		"DBUS_SESSION_BUS_ADDRESS=")
	_, status := runWrapper(t, wrapper, env, "https://failure.example/")

	if status != 3 {
		t.Fatalf("wrapper status = %d, want the brouter status 3", status)
	}
	log := readFileOrFatal(t, filepath.Join(home, "state", "brouter", "handler.log"))
	for _, want := range []string{"brouter diagnostic on stderr", "handler failure (status 3)"} {
		if !strings.Contains(log, want) {
			t.Errorf("handler log = %q, want it to contain %q", log, want)
		}
	}
}

func TestNixosHandlerDesktopEntryDeclaresSchemesAndSafeExec(t *testing.T) {
	fakeOut := "/run/current-system/sw"
	rendered := renderTemplate(t, "nixos/brouter-handler.desktop.in", "@packageBin@", fakeOut)

	entry := parseDesktopEntry(t, rendered)

	if entry["Type"] != "Application" {
		t.Errorf("Type = %q, want Application", entry["Type"])
	}
	wantMime := "x-scheme-handler/http;x-scheme-handler/https;"
	if entry["MimeType"] != wantMime {
		t.Errorf("MimeType = %q, want %q", entry["MimeType"], wantMime)
	}
	execLine := entry["Exec"]
	if !strings.HasPrefix(execLine, fakeOut+"/bin/brouter-handler ") {
		t.Errorf("Exec = %q, want an absolute path into the package", execLine)
	}
	if !strings.HasSuffix(execLine, " %u") {
		t.Errorf("Exec = %q, want the single-URL field code %%u", execLine)
	}
	if entry["Terminal"] != "false" {
		t.Errorf("Terminal = %q, want false", entry["Terminal"])
	}
	assertExecHasNoShellMetacharacters(t, execLine)

	// The entry must be valid where desktop-file-validate is available.
	if _, err := exec.LookPath("desktop-file-validate"); err == nil {
		dir := t.TempDir()
		path := filepath.Join(dir, "brouter-handler.desktop")
		if err := os.WriteFile(path, []byte(rendered), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command("desktop-file-validate", path).CombinedOutput(); err != nil {
			t.Errorf("desktop-file-validate rejected the entry: %v\n%s", err, out)
		}
	}
}

func parseDesktopEntry(t *testing.T, content string) map[string]string {
	t.Helper()
	entry := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if key, value, found := strings.Cut(line, "="); found && !strings.HasPrefix(key, "#") {
			entry[key] = value
		}
	}
	return entry
}

func readArgvFile(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

func readFileOrFatal(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	return string(data)
}

// assertExecHasNoShellMetacharacters locks safe Exec argument behavior:
// the desktop launcher must exec arguments directly, never interpret
// them through a shell.
func assertExecHasNoShellMetacharacters(t *testing.T, execLine string) {
	t.Helper()
	command := strings.TrimSuffix(execLine, " %u")
	for _, meta := range []string{";", "|", "&", "`", "$(", "&&"} {
		if strings.Contains(command, meta) {
			t.Errorf("Exec = %q contains shell metacharacter %q", execLine, meta)
		}
	}
}
