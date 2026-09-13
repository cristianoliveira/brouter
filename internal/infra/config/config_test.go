package config

import (
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
