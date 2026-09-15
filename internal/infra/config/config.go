// Package config loads and validates brouter's portable TOML
// configuration into pure domain routing inputs. Parsing lives here
// (infrastructure); internal/domain never sees TOML.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/cristianoliveira/brouter/internal/domain"
)

// KnownBrowsers are the browser identifiers a known target may use.
// Profiles are browser-local identifiers and are not portable across
// machines; command targets must be a single literal path.
var KnownBrowsers = map[string]bool{
	"brave":    true,
	"chrome":   true,
	"chromium": true,
	"edge":     true,
	"firefox":  true,
}

// shellMetacharacters must never appear in an executable command: targets
// are launched as argv, never through a shell.
const shellMetacharacters = "|&;<>()$`\\\"'\n\r"

// TargetDefinition describes one named destination from the config:
// either a known browser (with optional machine-local profile) or a
// generic executable path.
type TargetDefinition struct {
	Kind       string // "known" or "executable"
	Browser    string // known targets only
	Profile    string // known targets only; optional
	Command    string // executable targets only
	ProfileSet bool
}

type browserSpec struct {
	Browser string `toml:"browser"`
	Profile string `toml:"profile"`
	Command string `toml:"command"`
}

type ruleSpec struct {
	Name    string `toml:"name"`
	Matcher string `toml:"matcher"`
	Pattern string `toml:"pattern"`
	Target  string `toml:"target"`
}

// luaSpec is the opt-in scripted routing section (TASK-0025/0029):
// a single script file defining route(ctx). It carries no runtime
// selection and no other knobs; the script is resolved relative to the
// config file's directory when relative.
type luaSpec struct {
	Script string `toml:"script"`
}

type fileFormat struct {
	Default  string                 `toml:"default"`
	Browsers map[string]browserSpec `toml:"browsers"`
	Rules    []ruleSpec             `toml:"rules"`
	Lua      *luaSpec               `toml:"lua"`
}

// Config is the validated, domain-ready configuration.
type Config struct {
	Default domain.Target
	Targets map[string]TargetDefinition
	Rules   []domain.Rule

	// LuaScript is the absolute route-script path when the optional
	// [lua] section is present, or empty when scripting is not enabled.
	LuaScript string
}

// DefaultPath returns the one documented Unix user configuration
// location: brouter/config.toml beneath the user config root. The root
// is $XDG_CONFIG_HOME only when it is set to an absolute path;
// otherwise it is $HOME/.config — on macOS and Linux alike, so GUI
// invocations without shell startup environment resolve the same
// location. A relative XDG value falls back instead of failing; a
// missing HOME with no absolute XDG is a visible error. There are no
// implicit fallback locations (notably none in the macOS Application
// Support tree), no merging, and no automatic migration.
func DefaultPath() (string, error) {
	return defaultPath(os.Getenv("XDG_CONFIG_HOME"), os.Getenv("HOME"))
}

func defaultPath(xdgConfigHome, home string) (string, error) {
	if xdgConfigHome != "" && filepath.IsAbs(xdgConfigHome) {
		return filepath.Join(xdgConfigHome, "brouter", "config.toml"), nil
	}
	if home == "" {
		return "", fmt.Errorf(
			"cannot determine the user config directory: set $XDG_CONFIG_HOME to an absolute path, or set $HOME (got XDG=%q HOME=%q)",
			xdgConfigHome, home,
		)
	}
	return filepath.Join(home, ".config", "brouter", "config.toml"), nil
}

// LoadDefault loads the configuration from DefaultPath, the documented
// per-OS location. Explicit paths go through Load instead.
func LoadDefault() (*Config, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return Load(path)
}

// Load reads and validates the configuration file at path. Every error
// names the offending file, field, or rule; the file content is never
// dumped and url-regex patterns are redacted from errors: patterns often
// embed route secrets.
// readConfigFile reads the config with unstable-read rejection: the
// file is read twice and a disagreement between the two reads means a
// writer is mid-edit — the load is rejected visibly so a torn partial
// config is never served. Tests replace the seam.
var readConfigFile = os.ReadFile

func Load(path string) (*Config, error) {
	data, err := readConfigFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	second, err := readConfigFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if !bytes.Equal(data, second) {
		return nil, fmt.Errorf("%s: config changed while reading; retry", path)
	}

	var file fileFormat
	metadata, err := toml.Decode(string(data), &file)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid syntax: %w", path, err)
	}
	if unknown := metadata.Undecoded(); len(unknown) > 0 {
		names := make([]string, 0, len(unknown))
		for _, key := range unknown {
			names = append(names, key.String())
		}
		return nil, fmt.Errorf("%s: unknown field(s): %s", path, strings.Join(names, ", "))
	}

	cfg, err := validate(path, file)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func validate(path string, file fileFormat) (*Config, error) {
	fail := func(field string, format string, args ...any) error {
		return fmt.Errorf("%s: %s: %s", path, field, fmt.Sprintf(format, args...))
	}

	var errs []error

	if strings.TrimSpace(file.Default) == "" {
		errs = append(errs, fail("default", "default target is required"))
	}

	cfg := &Config{
		Default: domain.Target(file.Default),
		Targets: make(map[string]TargetDefinition, len(file.Browsers)),
	}

	errs = append(errs, validateBrowsers(path, file, cfg)...)
	rules, ruleErrs := validateRules(path, file.Rules, cfg.Targets)
	cfg.Rules = rules
	errs = append(errs, ruleErrs...)

	if err := validateLuaSection(file.Lua); err != nil {
		errs = append(errs, err)
	}

	if file.Default != "" {
		if _, ok := cfg.Targets[file.Default]; !ok {
			errs = append(errs, fail("default", "target %q is not defined in browsers", file.Default))
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	// The route script is resolved against the config file's directory
	// when relative, and re-read per URL by the bridge (never cached).
	if file.Lua != nil {
		script := file.Lua.Script
		if !filepath.IsAbs(script) {
			script = filepath.Join(filepath.Dir(path), script)
		}
		cfg.LuaScript = script
	}
	return cfg, nil
}

func validateLuaSection(spec *luaSpec) error {
	if spec != nil && strings.TrimSpace(spec.Script) == "" {
		return errors.New("lua.script: script must not be empty when [lua] is present")
	}
	return nil
}

// validateBrowsers validates every browser definition (sorted for
// deterministic diagnostics) and populates cfg.Targets.
func validateBrowsers(path string, file fileFormat, cfg *Config) []error {
	fail := func(field string, format string, args ...any) error {
		return fmt.Errorf("%s: %s: %s", path, field, fmt.Sprintf(format, args...))
	}
	var errs []error
	names := make([]string, 0, len(file.Browsers))
	for name := range file.Browsers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		def, err := validateTarget(name, file.Browsers[name])
		if err != nil {
			errs = append(errs, fail("browsers.\""+name+"\"", "%s", err))
			continue
		}
		cfg.Targets[name] = def
	}
	return errs
}

func validateRules(path string, specs []ruleSpec, targets map[string]TargetDefinition) ([]domain.Rule, []error) {
	var (
		rules []domain.Rule
		errs  []error
	)
	seen := map[string]bool{}
	for index, spec := range specs {
		if domain.MatcherKind(spec.Matcher) == domain.URLRegex {
			if _, err := regexp.Compile(spec.Pattern); err != nil {
				errs = append(errs, fmt.Errorf("%s: rules[%d].pattern (rule %q): invalid url regex: %v (pattern not shown)", path, index, spec.Name, err))
				continue
			}
		}
		rule, err := domain.NewRule(spec.Name, domain.MatcherKind(spec.Matcher), spec.Pattern, domain.Target(spec.Target))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: rules[%d]: %w", path, index, err))
			continue
		}
		if seen[rule.Name] {
			errs = append(errs, fmt.Errorf("%s: rules[%d]: duplicate rule name %q", path, index, rule.Name))
			continue
		}
		seen[rule.Name] = true
		if _, ok := targets[spec.Target]; !ok {
			errs = append(errs, fmt.Errorf("%s: rules[%d].target (rule %q): target %q is not defined in browsers", path, index, spec.Name, spec.Target))
			continue
		}
		rules = append(rules, rule)
	}
	return rules, errs
}

func validateTarget(name string, spec browserSpec) (TargetDefinition, error) {
	if strings.TrimSpace(name) == "" {
		return TargetDefinition{}, fmt.Errorf("target name must not be empty")
	}
	hasBrowser := strings.TrimSpace(spec.Browser) != ""
	hasCommand := strings.TrimSpace(spec.Command) != ""

	switch {
	case hasBrowser && hasCommand:
		return TargetDefinition{}, fmt.Errorf("target must be either a known browser or an executable, not both")
	case hasCommand:
		return validateExecutableTarget(spec)
	case hasBrowser:
		return validateKnownTarget(spec)
	default:
		return TargetDefinition{}, fmt.Errorf("target needs either a browser or a command")
	}
}

func validateExecutableTarget(spec browserSpec) (TargetDefinition, error) {
	if spec.Profile != "" {
		return TargetDefinition{}, fmt.Errorf("executable targets do not support a profile; profiles are a known-browser feature")
	}
	if strings.ContainsAny(spec.Command, shellMetacharacters) {
		return TargetDefinition{}, fmt.Errorf("command must be a single literal path without shell metacharacters")
	}
	return TargetDefinition{Kind: "executable", Command: spec.Command}, nil
}

func validateKnownTarget(spec browserSpec) (TargetDefinition, error) {
	if !KnownBrowsers[spec.Browser] {
		return TargetDefinition{}, fmt.Errorf("unknown browser %q (known: brave, chrome, chromium, edge, firefox)", spec.Browser)
	}
	if spec.Profile != "" {
		if err := validateProfileIdentifier(spec.Profile); err != nil {
			return TargetDefinition{}, err
		}
	}
	def := TargetDefinition{Kind: "known", Browser: spec.Browser}
	if spec.Profile != "" {
		def.Profile = spec.Profile
		def.ProfileSet = true
	}
	return def, nil
}

// validateProfileIdentifier constrains profiles to single directory
// identifiers inside the browser's user-data dir. Profiles are never
// free-form paths: separators would traverse outside it, a leading dash
// could be parsed as a browser flag, and dot names collide with
// filesystem specials. Display names are deliberately out of scope:
// users pass the identifier the browser created on disk (documented in
// the profile task).
func validateProfileIdentifier(profile string) error {
	if strings.ContainsAny(profile, `/\`) {
		return fmt.Errorf("profile %q must be a single directory identifier without path separators", profile)
	}
	if strings.HasPrefix(profile, "-") {
		return fmt.Errorf("profile %q must not start with a dash (it is passed as a browser argument)", profile)
	}
	if profile == "." || profile == ".." || strings.Contains(profile, "..") {
		return fmt.Errorf("profile %q must not contain dot-dot sequences", profile)
	}
	if strings.ContainsAny(profile, shellMetacharacters) {
		return fmt.Errorf("profile %q contains unsupported characters", profile)
	}
	return nil
}
