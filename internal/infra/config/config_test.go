package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianoliveira/brouter/internal/domain"
)

func loadFixture(t *testing.T, name string) (*Config, error) {
	t.Helper()
	return Load(filepath.Join("testdata", name))
}

func TestLoadValidMinimalConfig(t *testing.T) {
	// Given a minimal config with one known browser, when loaded, the
	// default target and target definition are populated.
	cfg, err := loadFixture(t, "valid-minimal.toml")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Default != domain.Target("personal") {
		t.Errorf("default = %q, want personal", cfg.Default)
	}
	personal, ok := cfg.Targets["personal"]
	if !ok {
		t.Fatalf("target personal missing: %+v", cfg.Targets)
	}
	if personal.Kind != "known" || personal.Browser != "chrome" || personal.Profile != "" {
		t.Errorf("target personal = %+v, want known chrome without profile", personal)
	}
	if len(cfg.Rules) != 0 {
		t.Errorf("rules = %+v, want none", cfg.Rules)
	}
}

func TestLoadValidConfigWithProfileAndExecutable(t *testing.T) {
	// Given a config with a profiled browser and a generic executable
	// target plus rules, when loaded, domain rules are built and usable.
	cfg, err := loadFixture(t, "valid-profile.toml")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Default != domain.Target("brave-work") {
		t.Errorf("default = %q, want brave-work", cfg.Default)
	}
	assertKnownTarget(t, cfg.Targets["brave-work"], "brave", "Work")
	assertExecutableTarget(t, cfg.Targets["legacy-tool"], "/usr/local/bin/legacy browser")

	if len(cfg.Rules) != 2 {
		t.Fatalf("rules = %d, want 2", len(cfg.Rules))
	}
	assertRule(t, cfg.Rules[0], domain.ExactHost, domain.Target("brave-work"))
	assertRule(t, cfg.Rules[1], domain.URLRegex, domain.Target("legacy-tool"))

	// The loaded rules are pure domain rules: they route without infra.
	router, err := domain.NewRouter(cfg.Rules, cfg.Default)
	if err != nil {
		t.Fatalf("domain router rejected loaded config: %v", err)
	}
	decision, err := router.Evaluate("https://company.example/page")
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !decision.Matched || decision.Target != domain.Target("brave-work") {
		t.Errorf("decision = %+v, want match to brave-work", decision)
	}
}

func assertKnownTarget(t *testing.T, def TargetDefinition, browser, profile string) {
	t.Helper()

	if def.Kind != "known" || def.Browser != browser || def.Profile != profile {
		t.Errorf("target = %+v, want known %s with profile %q", def, browser, profile)
	}
}

func assertExecutableTarget(t *testing.T, def TargetDefinition, command string) {
	t.Helper()

	if def.Kind != "executable" || def.Command != command {
		t.Errorf("target = %+v, want executable with literal path %q", def, command)
	}
}

func assertRule(t *testing.T, rule domain.Rule, kind domain.MatcherKind, target domain.Target) {
	t.Helper()

	if rule.Kind != kind || rule.Target != target {
		t.Errorf("rule = %+v, want %s to %q", rule, kind, target)
	}
}

func TestLoadRejectsMissingFile(t *testing.T) {
	// Given a path that does not exist, when loaded, the error names the
	// path instead of failing silently.
	_, err := Load(filepath.Join("testdata", "does-not-exist.toml"))
	if err == nil {
		t.Fatal("Load succeeded on missing file")
	}
	if !strings.Contains(err.Error(), "does-not-exist.toml") {
		t.Errorf("error = %q, want the missing path named", err)
	}
}

func TestLoadRejectsMalformedSyntax(t *testing.T) {
	// Given a file with broken TOML, when loaded, the error names the file
	// and the syntax problem without dumping the whole file.
	_, err := loadFixture(t, "invalid-syntax.toml")
	if err == nil {
		t.Fatal("Load succeeded on malformed TOML")
	}
	if !strings.Contains(err.Error(), "invalid-syntax.toml") {
		t.Errorf("error = %q, want the file named", err)
	}
	if strings.Contains(err.Error(), "browser = \"chrome\"") {
		t.Errorf("error = %q, want no full-file dump", err)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	// Given fields outside the schema, when loaded, each unknown field is
	// reported by its path.
	_, err := loadFixture(t, "unknown-fields.toml")
	if err == nil {
		t.Fatal("Load succeeded with unknown fields")
	}
	if !strings.Contains(err.Error(), "telemetry") {
		t.Errorf("error = %q, want unknown field telemetry named", err)
	}
	if !strings.Contains(err.Error(), "experimental") {
		t.Errorf("error = %q, want unknown field experimental named", err)
	}
}

func TestLoadRejectsDuplicateTargetDefinitions(t *testing.T) {
	// Given a target table defined twice, when loaded, TOML rejects it as
	// a syntax error (duplicates can never merge silently).
	if _, err := loadFixture(t, "duplicate-targets.toml"); err == nil {
		t.Fatal("Load succeeded with duplicate target tables")
	}
}

func TestLoadRejectsUnknownTargetReferences(t *testing.T) {
	// Given rules or default referencing undefined targets, when loaded,
	// the exact field or rule is named.
	_, err := loadFixture(t, "unknown-target-ref.toml")
	if err == nil {
		t.Fatal("Load succeeded with unknown target references")
	}
	if !strings.Contains(err.Error(), "missing-default") {
		t.Errorf("error = %q, want the default reference named", err)
	}
	if !strings.Contains(err.Error(), "points-nowhere") || !strings.Contains(err.Error(), "missing-target") {
		t.Errorf("error = %q, want the offending rule and target named", err)
	}
}

func TestLoadRejectsInvalidRegexAndMatcher(t *testing.T) {
	// Given rules with an uncompilable regex or unknown matcher, when
	// loaded, domain validation errors surface with rule context.
	if _, err := loadFixture(t, "invalid-regex.toml"); err == nil {
		t.Fatal("Load succeeded with invalid regex")
	} else if !strings.Contains(err.Error(), "bad-regex") {
		t.Errorf("error = %q, want the rule named", err)
	}

	if _, err := loadFixture(t, "unknown-matcher.toml"); err == nil {
		t.Fatal("Load succeeded with unknown matcher")
	} else if !strings.Contains(err.Error(), "fuzzy") {
		t.Errorf("error = %q, want the rule named", err)
	}
}

func TestLoadRejectsShellCommandStrings(t *testing.T) {
	// Given an executable target carrying a shell command string, when
	// loaded, it is rejected: executables are single literal paths.
	_, err := loadFixture(t, "shell-command.toml")
	if err == nil {
		t.Fatal("Load succeeded with shell command string")
	}
	if !strings.Contains(err.Error(), "shell") {
		t.Errorf("error = %q, want shell-string rejection", err)
	}
}

func TestLoadRejectsMissingDefault(t *testing.T) {
	// Given no default target, when loaded, the error names the field.
	_, err := loadFixture(t, "empty-default.toml")
	if err == nil {
		t.Fatal("Load succeeded without default")
	}
	if !strings.Contains(err.Error(), "default") {
		t.Errorf("error = %q, want the default field named", err)
	}
}

func TestLoadAggregatesTargetErrorsDeterministically(t *testing.T) {
	// Given multiple invalid targets, when loaded repeatedly, the
	// aggregated errors name them in sorted order every time — map
	// iteration must not leak nondeterminism into diagnostics.
	fixture := filepath.Join("testdata", "multiple-invalid-targets.toml")

	var first string
	for attempt := 0; attempt < 20; attempt++ {
		_, err := Load(fixture)
		if err == nil {
			t.Fatal("Load succeeded with invalid targets")
		}
		message := err.Error()
		if attempt == 0 {
			first = message
			continue
		}
		if message != first {
			t.Fatalf("error order is unstable:\nattempt 0: %s\nattempt %d: %s", first, attempt, message)
		}
	}

	aaa := strings.Index(first, "aaa-bad")
	zzz := strings.Index(first, "zzz-bad")
	if aaa == -1 || zzz == -1 {
		t.Fatalf("error = %q, want both invalid targets named", first)
	}
	if aaa > zzz {
		t.Errorf("errors not sorted by target name:\n%s", first)
	}
}

func TestLoadRedactsRegexPatternFromErrors(t *testing.T) {
	// Given an invalid url-regex whose pattern embeds a secret, when
	// loaded, the error names the rule and the syntax problem but never
	// echoes the pattern back.
	_, err := loadFixture(t, "invalid-regex-secret.toml")
	if err == nil {
		t.Fatal("Load succeeded with invalid regex")
	}
	if !strings.Contains(err.Error(), "leaky") || !strings.Contains(err.Error(), "invalid url regex") {
		t.Errorf("error = %q, want the rule named with invalid-regex reason", err)
	}
	if strings.Contains(err.Error(), "secret-hunter-123") {
		t.Errorf("error = %q, want the pattern redacted", err)
	}
}

func TestLoadRejectsExecutableTargetWithProfile(t *testing.T) {
	// Given an executable target carrying a profile, when loaded, it is
	// rejected instead of silently dropping the profile.
	_, err := loadFixture(t, "executable-with-profile.toml")
	if err == nil {
		t.Fatal("Load succeeded with executable target plus profile")
	}
	if !strings.Contains(err.Error(), "profile") || !strings.Contains(err.Error(), "executable") {
		t.Errorf("error = %q, want executable/profile conflict named", err)
	}
}

func TestLoadRejectsEmptyTargetName(t *testing.T) {
	// Given a target table with an empty name, when loaded, it is
	// rejected: target names identify destinations and must exist.
	_, err := loadFixture(t, "empty-target-name.toml")
	if err == nil {
		t.Fatal("Load succeeded with empty target name")
	}
	if !strings.Contains(err.Error(), "target name must not be empty") {
		t.Errorf("error = %q, want empty-name rejection", err)
	}
}

func TestLoadDefaultUsesTheDocumentedLocation(t *testing.T) {
	// Given a config written to the documented per-OS location, when
	// LoadDefault runs, it loads exactly that file; a missing file names
	// the expected path.
	configRoot := t.TempDir()
	t.Setenv("HOME", configRoot)
	t.Setenv("XDG_CONFIG_HOME", "")

	expected, err := DefaultPath()
	if err != nil {
		t.Skipf("no user config dir derivable: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(expected), 0o755); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join("testdata", "valid-minimal.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(expected, source, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault failed: %v", err)
	}
	if cfg.Default != domain.Target("personal") {
		t.Errorf("default = %q, want personal from the documented location", cfg.Default)
	}

	if err := os.Remove(expected); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDefault(); err == nil {
		t.Fatal("LoadDefault succeeded without a config file")
	} else if !strings.Contains(err.Error(), "config.toml") {
		t.Errorf("error = %q, want the expected path named", err)
	}
}

func TestDefaultPathResolvesPerContract(t *testing.T) {
	// The Unix config contract, identical on macOS and Linux:
	// --config wins (covered by the precedence tests); otherwise
	// $XDG_CONFIG_HOME/brouter/config.toml only when XDG is absolute;
	// a relative or unset XDG falls back to $HOME/.config; missing HOME
	// with no absolute XDG is a visible error. There is no fallback to
	// the old macOS Application Support location.
	cases := []struct {
		name    string
		xdg     string
		home    string
		want    string
		wantErr string
	}{
		{"unset xdg uses home", "", "/home/u", "/home/u/.config/brouter/config.toml", ""},
		{"absolute xdg wins", "/cfg", "/home/u", "/cfg/brouter/config.toml", ""},
		{"relative xdg falls back", "cfg", "/home/u", "/home/u/.config/brouter/config.toml", ""},
		{"dot-relative xdg falls back", "./cfg", "/home/u", "/home/u/.config/brouter/config.toml", ""},
		{"absolute xdg needs no home", "/cfg", "", "/cfg/brouter/config.toml", ""},
		{"missing home is visible", "", "", "", "XDG_CONFIG_HOME"},
		{"relative xdg with missing home is visible", "cfg", "", "", "HOME"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path, err := defaultPath(tc.xdg, tc.home)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("defaultPath(%q, %q) succeeded, want visible failure", tc.xdg, tc.home)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error = %q, want it to mention %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("defaultPath(%q, %q) failed: %v", tc.xdg, tc.home, err)
			}
			if path != tc.want {
				t.Errorf("defaultPath(%q, %q) = %q, want %q", tc.xdg, tc.home, path, tc.want)
			}
		})
	}
}

func TestDefaultPathFollowsTheEnvironment(t *testing.T) {
	// The environment-facing entry point applies the same contract.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath failed: %v", err)
	}
	if want := filepath.Join(home, ".config", "brouter", "config.toml"); path != want {
		t.Errorf("default path = %q, want %q", path, want)
	}

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	path, err = DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath failed: %v", err)
	}
	if want := filepath.Join(xdg, "brouter", "config.toml"); path != want {
		t.Errorf("default path = %q, want %q", path, want)
	}
}

func writeTOML(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadRejectsUnsafeProfileIdentifiers(t *testing.T) {
	// Given profile values that would traverse paths, inject flags, or
	// collide with special directories, when loaded, each is rejected with
	// a visible reason. Profiles are directory identifiers inside the
	// browser's user-data dir, never free-form paths.
	for name, profile := range map[string]string{
		"path separator":        "Work/extra",
		"windows separator":     `Work\extra`,
		"flag injection":        "-P evil",
		"parent directory":      "..",
		"current directory":     ".",
		"equals-form injection": "--profile-directory=evil",
	} {
		t.Run(name, func(t *testing.T) {
			path := writeTOML(t, fmt.Sprintf(`
default = "work"

[browsers.work]
browser = "brave"
profile = %q
`, profile))

			_, err := Load(path)
			if err == nil {
				t.Fatalf("profile %q accepted, want rejection", profile)
			}
			if !strings.Contains(err.Error(), "profile") {
				t.Errorf("error %q does not mention the profile", err)
			}
		})
	}
}

func TestLoadAcceptsSafeProfileIdentifiers(t *testing.T) {
	// Given ordinary profile directory names (as shown by the browser's
	// profile list), when loaded, they are accepted and marked as set.
	path := writeTOML(t, `
default = "work"

[browsers.work]
browser = "brave"
profile = "Profile 1"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	assertKnownTarget(t, cfg.Targets["work"], "brave", "Profile 1")
	if !cfg.Targets["work"].ProfileSet {
		t.Error("ProfileSet = false, want true")
	}
}

// route_command is validated structurally only: never resolved, never
// executed. Empty argv, empty executable, or empty elements fail.
func TestValidateRouteCommandStructureOnly(t *testing.T) {
	cases := []struct {
		name    string
		section string
		ok      bool
		wantErr string
	}{
		{"absent section is fine", "", true, ""},
		{"valid argv", "\n[route_command]\ncommand = [\"/bin/sh\", \"/p/r.sh\"]\n", true, ""},
		{"empty argv", "\n[route_command]\ncommand = []\n", false, "non-empty argv array"},
		{"empty executable", "\n[route_command]\ncommand = [\"\"]\n", false, "non-empty argv array"},
		{"empty element", "\n[route_command]\ncommand = [\"/bin/sh\", \"\"]\n", false, "argv[1] is empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			content := "default = \"personal\"\n\n[browsers.personal]\nbrowser = \"brave\"\n" + tc.section
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Load(path)
			if tc.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.ok && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

// The reserved @default line means explicit defer; while route_command
// is active it cannot also be a configured browser ID.
func TestValidateRouteCommandDefaultCollision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := `default = "app"

[browsers.app]
browser = "safari"

[browsers.@default]
browser = "chromium"

[route_command]
command = ["/bin/sh", "/p/r.sh"]
`
	// A raw "@default" table key needs quoting in TOML.
	content = strings.Replace(content, "[browsers.@default]", "[browsers.\"@default\"]", 1)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "reserved for explicit defer") {
		t.Fatalf("err = %v, want the @default collision error", err)
	}
}

// A config rewritten mid-read is rejected visibly: the two snapshot
// reads disagree, so Load fails instead of serving an observed-unstable
// file. (Bounded checks cannot detect a stationary partial write.)
func TestLoadRejectsObservedConfigChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("stable"), 0o644); err != nil {
		t.Fatal(err)
	}
	original := readConfigFile
	calls := 0
	readConfigFile = func(string) ([]byte, error) {
		calls++
		if calls == 1 {
			return []byte("torn"), nil
		}
		return []byte("rewritten"), nil
	}
	defer func() { readConfigFile = original }()

	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "changed while reading") {
		t.Fatalf("observed change must be rejected visibly, got err=%v", err)
	}
}
