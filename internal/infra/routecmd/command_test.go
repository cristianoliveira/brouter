package routecmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestResolveCommandUsesLogicalConfigDirectory(t *testing.T) {
	// Given a symlinked config path and a colocated route.py symlink, when
	// resolving ./route.py, the logical config directory is preferred over
	// the symlink target and the process working directory.
	root := t.TempDir()
	logicalDir := filepath.Join(root, "config")
	targetDir := filepath.Join(root, "dotfiles")
	for _, dir := range []string{logicalDir, targetDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(logicalDir, "config.toml")
	if err := os.Symlink(filepath.Join(targetDir, "config.toml"), configPath); err != nil {
		t.Fatal(err)
	}

	got, err := resolveCommand([]string{"./route.py", "--flag with spaces"}, configPath)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(logicalDir, "route.py"), "--flag with spaces"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolved command = %#v, want %#v", got, want)
	}
}

func TestResolveCommandPreservesAbsoluteAndBareExecutables(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	cases := []struct {
		name string
		argv []string
	}{
		{name: "absolute", argv: []string{"/usr/bin/python3", "route.py"}},
		{name: "bare PATH command", argv: []string{"python3", "route.py"}},
		{name: "tilde remains literal", argv: []string{"~/.local/bin/router"}},
		{name: "environment variable remains literal", argv: []string{"$HOME/router"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveCommand(tc.argv, configPath)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got[1:], tc.argv[1:]) {
				t.Fatalf("arguments = %#v, want %#v", got[1:], tc.argv[1:])
			}
			if tc.name == "absolute" || tc.name == "bare PATH command" {
				if got[0] != tc.argv[0] {
					t.Errorf("executable = %q, want unchanged %q", got[0], tc.argv[0])
				}
				return
			}
			want := filepath.Join(filepath.Dir(configPath), tc.argv[0])
			if got[0] != want {
				t.Errorf("executable = %q, want literal config-relative path %q", got[0], want)
			}
		})
	}
}

func TestResolveCommandDoesNotMutateInput(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	argv := []string{"./route.py", "--keep", "value"}
	original := append([]string(nil), argv...)

	if _, err := resolveCommand(argv, configPath); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(argv, original) {
		t.Fatalf("input argv mutated: %#v, want %#v", argv, original)
	}
}

// writeSymlinkedScriptFixture builds a config.toml symlink and a
// colocated route.py symlink whose targets live in a separate
// directory, then moves the process into another working directory.
func writeSymlinkedScriptFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	logicalDir := filepath.Join(root, "config")
	targetDir := filepath.Join(root, "dotfiles")
	for _, dir := range []string{logicalDir, targetDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(logicalDir, "config.toml")
	targetConfig := filepath.Join(targetDir, "config.toml")
	if err := os.WriteFile(targetConfig, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetConfig, configPath); err != nil {
		t.Fatal(err)
	}
	targetScript := filepath.Join(targetDir, "route.py")
	script := []byte("#!/bin/sh\n[ \"$1\" = \"--flag with spaces\" ] || exit 9\n[ \"$2\" = \"value\" ] || exit 9\nprintf 'work\\n'\n")
	if err := os.WriteFile(targetScript, script, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(targetScript, filepath.Join(logicalDir, "route.py")); err != nil {
		t.Fatal(err)
	}

	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	otherDir := filepath.Join(root, "other")
	if err := os.Mkdir(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(otherDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(workingDir) })
	return configPath
}

func TestCommanderExecutesRelativeScriptBesideSymlinkedConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}

	// Given a config symlink and a colocated route.py symlink, when the
	// process starts from another directory, the relative executable is
	// found beside the logical config and receives untouched arguments.
	configPath := writeSymlinkedScriptFixture(t)

	commander := &Commander{
		Command:    []string{"./route.py", "--flag with spaces", "value"},
		ConfigPath: configPath,
	}
	result, err := commander.Decide(context.Background(), "https://example.com/")
	if err != nil {
		t.Fatal(err)
	}
	if result.Route != "work" || result.Defer {
		t.Fatalf("result = %+v, want work", result)
	}
}

func TestCommanderReportsMissingAndNonExecutableRelativeScripts(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode os.FileMode
	}{
		{name: "missing", mode: 0},
		{name: "non-executable", mode: 0o644},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			configPath := filepath.Join(root, "config.toml")
			if err := os.WriteFile(configPath, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			if tc.mode != 0 {
				if err := os.WriteFile(filepath.Join(root, "route.py"), []byte("#!/bin/sh\nprintf 'work\\n'\n"), tc.mode); err != nil {
					t.Fatal(err)
				}
			}

			commander := &Commander{Command: []string{"./route.py"}, ConfigPath: configPath}
			_, err := commander.Decide(context.Background(), "https://example.com/")
			if !errors.Is(err, ErrCommandUnavailable) {
				t.Fatalf("error = %v, want %v", err, ErrCommandUnavailable)
			}
		})
	}
}
