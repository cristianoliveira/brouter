package scripts

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func runReleaseHelper(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{"scripts/release-artifacts.sh"}, args...)...)
	cmd.Dir = ".."
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func TestReleaseTagValidation(t *testing.T) {
	for _, test := range []struct {
		tag     string
		version string
		valid   bool
	}{
		{tag: "v0.1.0", version: "0.1.0", valid: true},
		{tag: "v12.34.567", version: "12.34.567", valid: true},
		{tag: "1.2.3"},
		{tag: "v1.2"},
		{tag: "v1.2.3-rc1"},
		{tag: "v1.2.3;touch /tmp/release-pwned"},
	} {
		t.Run(test.tag, func(t *testing.T) {
			output, err := runReleaseHelper(t, "validate-tag", test.tag)
			if test.valid {
				if err != nil {
					t.Fatalf("validation failed: %v\n%s", err, output)
				}
				if strings.TrimSpace(output) != test.version {
					t.Errorf("output = %q, want %q", output, test.version)
				}
				return
			}
			if err == nil {
				t.Fatalf("validation succeeded for malformed tag: %q", test.tag)
			}
			if !strings.Contains(output, "must match vX.Y.Z") {
				t.Errorf("output = %q, want the validation diagnosis", output)
			}
		})
	}
}

func TestReleaseRejectsUnsupportedTargetAndMissingArtifacts(t *testing.T) {
	if output, err := runReleaseHelper(t, "build", "v1.2.3", "windows-amd64", t.TempDir()); err == nil {
		t.Fatalf("unsupported target succeeded: %s", output)
	}
	if output, err := runReleaseHelper(t, "manifest", "v1.2.3", t.TempDir()); err == nil {
		t.Fatalf("incomplete manifest succeeded: %s", output)
	}
}

func TestReleaseManifestIsSortedByFilename(t *testing.T) {
	// Given all supported archive names, when the manifest is generated, its
	// records are sorted by filename rather than build-job completion order.
	dir := t.TempDir()
	names := []string{
		"brouter-v1.2.3-linux-x86_64.tar.gz",
		"brouter-v1.2.3-linux-aarch64.tar.gz",
		"brouter-v1.2.3-darwin-arm64.tar.gz",
		"brouter-handler-v1.2.3-darwin-arm64.tar.gz",
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	output, err := runReleaseHelper(t, "manifest", "v1.2.3", dir)
	if err != nil {
		t.Fatalf("manifest failed: %v\n%s", err, output)
	}
	manifest, err := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	var actual []string
	for _, line := range strings.Split(strings.TrimSpace(string(manifest)), "\n") {
		actual = append(actual, strings.Fields(line)[1])
	}
	expected := append([]string(nil), names...)
	sort.Strings(expected)
	if strings.Join(actual, "\n") != strings.Join(expected, "\n") {
		t.Errorf("manifest filenames = %v, want sorted %v", actual, expected)
	}
}

func TestLinuxArtifactIsStampedAndDeterministic(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	for _, outputDir := range []string{first, second} {
		output, err := runReleaseHelper(t, "build", "v1.2.3", "linux-x86_64", outputDir)
		if err != nil {
			t.Fatalf("build failed: %v\n%s", err, output)
		}
	}

	archiveName := "brouter-v1.2.3-linux-x86_64.tar.gz"
	firstBytes, err := os.ReadFile(filepath.Join(first, archiveName))
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := os.ReadFile(filepath.Join(second, archiveName))
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(firstBytes)) != fmt.Sprintf("%x", sha256.Sum256(secondBytes)) {
		t.Fatal("identical release builds produced different archives")
	}

	entries := readArchive(t, filepath.Join(first, archiveName))
	root := "brouter-1.2.3-linux-x86_64/"
	if entries[root+"VERSION"] != "1.2.3\n" {
		t.Errorf("VERSION = %q, want 1.2.3", entries[root+"VERSION"])
	}
	if _, ok := entries[root+"brouter"]; !ok {
		t.Errorf("archive entries = %v, want stamped CLI", entries)
	}
}

func readArchive(t *testing.T, path string) map[string]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer compressed.Close()
	archive := tar.NewReader(compressed)
	entries := make(map[string]string)
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		contents, err := io.ReadAll(archive)
		if err != nil {
			t.Fatal(err)
		}
		entries[header.Name] = string(contents)
	}
	return entries
}
