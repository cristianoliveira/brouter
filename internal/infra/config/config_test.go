package config

import (
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

func TestDefaultPathUsesUserConfigDir(t *testing.T) {
	// Given the platform user config directory, when the default path is
	// computed, it is brouter/config.toml beneath it; without a config
	// directory the failure is visible.
	path, err := DefaultPath()
	if err != nil {
		t.Skipf("no user config dir on this host: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(path), "brouter/config.toml") {
		t.Errorf("default path = %q, want brouter/config.toml beneath the user config dir", path)
	}

	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	if _, err := DefaultPath(); err == nil {
		t.Error("DefaultPath succeeded without any config dir, want visible failure")
	}
}
