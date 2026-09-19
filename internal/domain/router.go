// Package domain holds pure routing decisions for brouter: rules, URL
// evaluation, and the data an explain/open command renders. Nothing here
// touches the filesystem, processes, or the network.
package domain

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/cristianoliveira/brouter/internal/domain/installedwebapp"
)

// Target names a browser destination configured elsewhere.
type Target string

// MatcherKind selects how a rule's pattern is evaluated.
type MatcherKind string

const (
	// ExactHost matches when the URL host equals the pattern exactly
	// after normalization (case, one trailing dot, ports stripped).
	ExactHost MatcherKind = "exact-host"

	// Subdomain matches the bare pattern host and any deeper subdomain
	// at label boundaries: a.example.com matches example.com,
	// evilaexample.com does not.
	Subdomain MatcherKind = "subdomain"

	// URLRegex matches the original URL string, exactly as received,
	// with a Go regexp (RE2: no lookaround, no backreferences, and
	// case-sensitive unless the pattern opts in).
	URLRegex MatcherKind = "url-regex"
)

// Rule routes URLs whose matcher agrees with the pattern to one target.
// Construct with NewRule; the zero value is not usable.
type Rule struct {
	Name    string
	Kind    MatcherKind
	Pattern string
	Target  Target

	regex *regexp.Regexp
}

// NewRule validates and compiles a rule. URL-regex patterns are compiled
// here so configuration errors surface at load time, not per request.
func NewRule(name string, kind MatcherKind, pattern string, target Target) (Rule, error) {
	if strings.TrimSpace(name) == "" {
		return Rule{}, fmt.Errorf("rule name must not be empty")
	}
	if target == "" {
		return Rule{}, fmt.Errorf("rule %q has an empty target", name)
	}

	var re *regexp.Regexp
	switch kind {
	case ExactHost, Subdomain:
		if err := validateHostPattern(pattern); err != nil {
			return Rule{}, fmt.Errorf("rule %q has an invalid host pattern %q: %w", name, pattern, err)
		}
	case URLRegex:
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return Rule{}, fmt.Errorf("rule %q has an invalid url regex %q: %w", name, pattern, err)
		}
		re = compiled
	default:
		return Rule{}, fmt.Errorf("rule %q has an unknown matcher kind %q", name, kind)
	}

	return Rule{Name: name, Kind: kind, Pattern: pattern, Target: target, regex: re}, nil
}

func validateHostPattern(pattern string) error {
	host := strings.ToLower(strings.TrimSuffix(pattern, "."))
	if host == "" {
		return fmt.Errorf("host pattern must not be empty")
	}
	if strings.ContainsAny(host, ":/ \t\r\n") {
		return fmt.Errorf("host pattern must be a bare hostname without port or separators")
	}
	for label := range strings.SplitSeq(host, ".") {
		if err := validateHostLabel(label); err != nil {
			return err
		}
	}
	return nil
}

func validateHostLabel(label string) error {
	if label == "" {
		return fmt.Errorf("host pattern has an empty label")
	}
	if len(label) > 63 {
		return fmt.Errorf("host pattern label %q is longer than 63 bytes", label)
	}
	for _, r := range label {
		if !isAllowedHostRune(r) {
			return fmt.Errorf("host pattern contains invalid character %q", r)
		}
	}
	return nil
}

// isAllowedHostRune accepts lowercase DNS letters, digits, hyphens, and
// any non-ASCII rune: internationalized labels are matched as received,
// with no punycode conversion.
func isAllowedHostRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
		return true
	case r >= 0x80:
		return true
	default:
		return false
	}
}

// Reason records one evaluated rule and what it decided, in evaluation
// order.
type Reason struct {
	Rule    string
	Kind    MatcherKind
	Pattern string
	Matched bool
	Detail  string
}

// Decision is the full outcome of one evaluation. URL preserves the
// original input for forwarding; Target is the rule target on a match and
// the router default on fallback.
type Decision struct {
	URL          string
	Target       Target
	Matched      bool
	MatchedRule  string
	Source       string
	InstalledApp *installedwebapp.Entry
	Reasons      []Reason
	Skipped      []string
}

// InvalidURLError reports an input the router will never route.
type InvalidURLError struct {
	Input  string
	Reason string
}

func (e *InvalidURLError) Error() string {
	// The raw input is redacted: URLs frequently embed tokens and other
	// secrets, and error output ends up in logs and CI transcripts. The
	// reason carries the actionable part (scheme, parse problem).
	return fmt.Sprintf("invalid url (input redacted): %s", e.Reason)
}

// Router evaluates URLs against ordered rules with first-match-wins
// semantics and an explicit default target for fallback.
type Router struct {
	rules         []Rule
	defaultTarget Target
	installed     *installedwebapp.Catalog
}

// NewRouter validates rules (through NewRule, so hand-built rules are
// normalized and compiled), rejects duplicate rule names, and requires a
// non-empty default target.
func NewRouter(rules []Rule, defaultTarget Target, catalogs ...installedwebapp.Catalog) (*Router, error) {
	if defaultTarget == "" {
		return nil, fmt.Errorf("router needs a non-empty default target")
	}

	normalized := make([]Rule, 0, len(rules))
	seen := make(map[string]bool, len(rules))
	for _, rule := range rules {
		valid, err := NewRule(rule.Name, rule.Kind, rule.Pattern, rule.Target)
		if err != nil {
			return nil, err
		}
		if seen[valid.Name] {
			return nil, fmt.Errorf("duplicate rule name %q", valid.Name)
		}
		seen[valid.Name] = true
		normalized = append(normalized, valid)
	}

	var catalog *installedwebapp.Catalog
	if len(catalogs) > 0 {
		if len(catalogs) > 1 {
			return nil, fmt.Errorf("router accepts at most one installed web app catalog")
		}
		catalogCopy := catalogs[0]
		catalog = &catalogCopy
	}
	return &Router{rules: normalized, defaultTarget: defaultTarget, installed: catalog}, nil
}

// Evaluate decides the target for one URL. Host matching is
// case-insensitive, ignores ports and one trailing dot, and compares
// parsed labels; internationalized hosts are compared as received, with
// no IDN/punycode conversion. URL-regex rules evaluate the original
// input exactly as received.
func (r *Router) Evaluate(rawURL string) (Decision, error) {
	parsed, host, err := routeURL(rawURL)
	if err != nil {
		return Decision{}, err
	}
	decision := Decision{URL: rawURL}
	if r.evaluateRules(&decision, host, rawURL) {
		return decision, nil
	}
	if r.evaluateInstalled(&decision, parsed, rawURL) {
		return decision, nil
	}
	decision.Target = r.defaultTarget
	decision.Source = "configured-default"
	return decision, nil
}

func routeURL(rawURL string) (url.URL, string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return url.URL{}, "", &InvalidURLError{Input: rawURL, Reason: fmt.Sprintf("malformed url: %s", sanitizeParseError(err, rawURL))}
	}
	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "http", "https":
	case "":
		return url.URL{}, "", &InvalidURLError{Input: rawURL, Reason: "malformed url: missing scheme"}
	default:
		return url.URL{}, "", &InvalidURLError{Input: rawURL, Reason: fmt.Sprintf("unsupported scheme %q (only http and https are routed)", scheme)}
	}
	host := normalizeHost(parsed.Hostname())
	if host == "" {
		return url.URL{}, "", &InvalidURLError{Input: rawURL, Reason: "empty host"}
	}
	return *parsed, host, nil
}

func (r *Router) evaluateRules(decision *Decision, host, rawURL string) bool {
	for _, rule := range r.rules {
		matched, detail := ruleMatches(rule, host, rawURL)
		decision.Reasons = append(decision.Reasons, Reason{
			Rule: rule.Name, Kind: rule.Kind, Pattern: rule.Pattern,
			Matched: matched, Detail: detail,
		})
		if !matched {
			continue
		}
		decision.Target = rule.Target
		decision.Matched = true
		decision.MatchedRule = rule.Name
		decision.Source = "static-rule"
		for _, remaining := range r.rules[len(decision.Reasons):] {
			decision.Skipped = append(decision.Skipped, remaining.Name)
		}
		return true
	}
	return false
}

func (r *Router) evaluateInstalled(decision *Decision, parsed url.URL, rawURL string) bool {
	if r.installed == nil || parsed.User != nil {
		return false
	}
	entry, matched, err := r.installed.Match(rawURL)
	if err != nil || !matched {
		return false
	}
	decision.Target = Target(entry.ID)
	decision.Matched = true
	decision.Source = "installed-app"
	decision.InstalledApp = &entry
	return true
}

// sanitizeParseError strips the quoted input that net/url embeds in its
// parse errors, so raw URLs (which may carry tokens) are never echoed in
// diagnostics.
func sanitizeParseError(err error, rawURL string) string {
	message := err.Error()
	if rawURL != "" {
		message = strings.ReplaceAll(message, strconv.Quote(rawURL)+": ", "")
	}
	return message
}

func ruleMatches(rule Rule, host, rawURL string) (bool, string) {
	pattern := normalizeHost(rule.Pattern)
	switch rule.Kind {
	case ExactHost:
		return matchExactHost(host, pattern)
	case Subdomain:
		return matchSubdomain(host, pattern)
	case URLRegex:
		return matchURLRegex(rule.regex, rawURL)
	default:
		return false, fmt.Sprintf("unknown matcher kind %q", rule.Kind)
	}
}

func matchExactHost(host, pattern string) (bool, string) {
	if host == pattern {
		return true, fmt.Sprintf("host %q equals pattern", host)
	}
	return false, fmt.Sprintf("host %q does not equal pattern %q", host, pattern)
}

func matchSubdomain(host, pattern string) (bool, string) {
	if host == pattern {
		return true, fmt.Sprintf("host %q equals pattern", host)
	}
	if strings.HasSuffix(host, "."+pattern) {
		return true, fmt.Sprintf("host %q is a subdomain of %q", host, pattern)
	}
	return false, fmt.Sprintf("host %q is not %q or a subdomain of it", host, pattern)
}

func matchURLRegex(re *regexp.Regexp, rawURL string) (bool, string) {
	if re != nil && re.MatchString(rawURL) {
		return true, "url matches the rule regex"
	}
	return false, "url does not match the rule regex"
}

// normalizeHost lowercases and strips one trailing FQDN dot. Ports are
// already gone by the time a parsed Hostname reaches this. IDN hosts stay
// in the encoding they arrived in: no punycode conversion is performed.
func normalizeHost(host string) string {
	return strings.ToLower(strings.TrimSuffix(host, "."))
}
