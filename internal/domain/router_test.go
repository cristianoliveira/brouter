package domain

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func mustRouter(t *testing.T, rules []Rule, fallback Target) *Router {
	t.Helper()

	router, err := NewRouter(rules, fallback)
	if err != nil {
		t.Fatalf("router construction failed: %v", err)
	}
	return router
}

func mustRule(t testing.TB, name string, kind MatcherKind, pattern string, target Target) Rule {
	t.Helper()

	rule, err := NewRule(name, kind, pattern, target)
	if err != nil {
		t.Fatalf("NewRule(%q, %q, %q) failed: %v", name, kind, pattern, err)
	}
	return rule
}

func TestExactHostMatchesWholeHostOnly(t *testing.T) {
	// Given an exact-host rule for company.example, when URLs are
	// evaluated, only that whole host (any case, any port, optional
	// trailing dot) matches; lookalike hosts never do.
	router := mustRouter(t, []Rule{
		mustRule(t, "work", ExactHost, "company.example", "Brave Work"),
	}, Target("Chrome Personal"))

	cases := []struct {
		name    string
		url     string
		matched bool
	}{
		{name: "same host matches", url: "https://company.example/path", matched: true},
		{name: "host case is ignored", url: "https://COMPANY.example/path", matched: true},
		{name: "port is ignored for matching", url: "https://company.example:8443/path", matched: true},
		{name: "trailing dot is ignored", url: "https://company.example./path", matched: true},
		{name: "different host falls back", url: "https://other.example/path", matched: false},
		{name: "longer host is not a substring match", url: "https://company.example.evil.test/path", matched: false},
		{name: "shared suffix is not a substring match", url: "https://evilcompany.example/path", matched: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision, err := router.Evaluate(tc.url)
			if err != nil {
				t.Fatalf("Evaluate(%q) failed: %v", tc.url, err)
			}
			if decision.Matched != tc.matched {
				t.Errorf("matched = %v, want %v (target %q)", decision.Matched, tc.matched, decision.Target)
			}
			if tc.matched && decision.Target != "Brave Work" {
				t.Errorf("target = %q, want rule target", decision.Target)
			}
			if !tc.matched && decision.Target != "Chrome Personal" {
				t.Errorf("target = %q, want default", decision.Target)
			}
		})
	}
}

func TestSubdomainMatchesAtLabelBoundaries(t *testing.T) {
	// Given a subdomain rule for example.com, when URLs are evaluated,
	// the bare host and any deeper subdomain match at label boundaries;
	// shared-suffix and shallower hosts never do.
	router := mustRouter(t, []Rule{
		mustRule(t, "work", Subdomain, "example.com", "Brave Work"),
	}, Target("Chrome Personal"))

	cases := []struct {
		name    string
		url     string
		matched bool
	}{
		{name: "bare host matches", url: "https://example.com/", matched: true},
		{name: "one level matches", url: "https://www.example.com/", matched: true},
		{name: "deep subdomain matches", url: "https://a.b.example.com/", matched: true},
		{name: "host case is ignored", url: "https://WWW.EXAMPLE.COM/", matched: true},
		{name: "sibling host is not a match", url: "https://notexample.com/", matched: false},
		{name: "prefix without label boundary is not a match", url: "https://evilexample.com/", matched: false},
		{name: "longer host is not a match", url: "https://example.com.evil.test/", matched: false},
		{name: "shallower host does not match deeper pattern", url: "https://com/", matched: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision, err := router.Evaluate(tc.url)
			if err != nil {
				t.Fatalf("Evaluate(%q) failed: %v", tc.url, err)
			}
			if decision.Matched != tc.matched {
				t.Errorf("matched = %v, want %v", decision.Matched, tc.matched)
			}
		})
	}
}

func TestURLRegexEvaluatesOriginalURL(t *testing.T) {
	// Given a full-URL rule, when URLs are evaluated, the regex sees the
	// original input (scheme, query, case) exactly as received, and is
	// case-sensitive by default.
	router := mustRouter(t, []Rule{
		mustRule(t, "tickets", URLRegex, `^https://tickets\.internal\.example/\?year=2026`, "Brave Work"),
	}, Target("Chrome Personal"))

	cases := []struct {
		name    string
		url     string
		matched bool
	}{
		{name: "original url with extra query matches", url: "https://tickets.internal.example/?year=2026&q=x", matched: true},
		{name: "regex is case-sensitive", url: "https://TICKETS.internal.example/?year=2026", matched: false},
		{name: "different query does not match", url: "https://tickets.internal.example/?year=2025", matched: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision, err := router.Evaluate(tc.url)
			if err != nil {
				t.Fatalf("Evaluate(%q) failed: %v", tc.url, err)
			}
			if decision.Matched != tc.matched {
				t.Errorf("matched = %v, want %v", decision.Matched, tc.matched)
			}
			if decision.URL != tc.url {
				t.Errorf("decision URL = %q, want the original %q", decision.URL, tc.url)
			}
		})
	}
}

func TestFirstMatchWinsAndSkipsRest(t *testing.T) {
	// Given overlapping rules, when a URL matches the first, evaluation
	// stops: the first target wins and later rules are recorded as
	// skipped, in order.
	router := mustRouter(t, []Rule{
		mustRule(t, "first", ExactHost, "shared.example", "Brave Work"),
		mustRule(t, "second", Subdomain, "shared.example", "Chrome Work"),
		mustRule(t, "third", URLRegex, `^https://shared\.example`, "Firefox Work"),
	}, Target("Chrome Personal"))

	decision, err := router.Evaluate("https://shared.example/page")
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if decision.Target != "Brave Work" || !decision.Matched {
		t.Errorf("target = %q matched = %v, want first rule target", decision.Target, decision.Matched)
	}
	if decision.MatchedRule != "first" {
		t.Errorf("matched rule = %q, want first", decision.MatchedRule)
	}
	if want := []string{"second", "third"}; !reflect.DeepEqual(decision.Skipped, want) {
		t.Errorf("skipped = %v, want %v", decision.Skipped, want)
	}
	if len(decision.Reasons) != 1 || decision.Reasons[0].Rule != "first" || !decision.Reasons[0].Matched {
		t.Errorf("reasons = %+v, want exactly the winning first rule", decision.Reasons)
	}
}

func TestAllRulesEvaluatedWhenNothingMatches(t *testing.T) {
	// Given rules that do not match, when a URL is evaluated, every rule
	// gets an ordered no-match reason and nothing is skipped.
	router := mustRouter(t, []Rule{
		mustRule(t, "a", ExactHost, "a.example", "Brave Work"),
		mustRule(t, "b", Subdomain, "b.example", "Chrome Work"),
	}, Target("Chrome Personal"))

	decision, err := router.Evaluate("https://other.example/")
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if decision.Matched || decision.Target != "Chrome Personal" {
		t.Errorf("decision = %+v, want fallback to default", decision)
	}
	if len(decision.Reasons) != 2 || decision.Reasons[0].Rule != "a" || decision.Reasons[1].Rule != "b" {
		t.Errorf("reasons = %+v, want ordered a then b", decision.Reasons)
	}
	if decision.Skipped != nil {
		t.Errorf("skipped = %v, want none", decision.Skipped)
	}
}

func TestEvaluateRejectsUnsupportedAndMalformedURLs(t *testing.T) {
	// Given URLs outside the supported scheme set or malformed beyond
	// parsing, when evaluated, they are rejected with actionable reasons.
	router := mustRouter(t, []Rule{
		mustRule(t, "work", ExactHost, "company.example", "Brave Work"),
	}, Target("Chrome Personal"))

	cases := []struct {
		name string
		url  string
		want string
	}{
		{name: "ftp is unsupported", url: "ftp://company.example/file", want: `unsupported scheme "ftp"`},
		{name: "javascript is unsupported", url: "javascript:alert(1)", want: `unsupported scheme "javascript"`},
		{name: "file is unsupported", url: "file:///etc/hosts", want: `unsupported scheme "file"`},
		{name: "empty input is malformed", url: "", want: "malformed"},
		{name: "text without scheme is malformed", url: "not a url", want: "malformed"},
		{name: "schemeless host is malformed", url: "company.example/path", want: "malformed"},
		{name: "unparseable host is malformed", url: "http://[bad/path", want: "malformed"},
		{name: "empty host cannot be routed", url: "http:///path", want: "empty host"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := router.Evaluate(tc.url)
			if err == nil {
				t.Fatalf("Evaluate(%q) accepted, want rejection", tc.url)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
			var invalid *InvalidURLError
			if !errors.As(err, &invalid) {
				t.Errorf("error = %T, want *InvalidURLError", err)
			}
		})
	}
}

func TestUppercaseSchemeIsSupported(t *testing.T) {
	// Given the scheme is case-insensitive per RFC 3986, when an uppercase
	// scheme arrives, it routes normally.
	router := mustRouter(t, []Rule{
		mustRule(t, "work", ExactHost, "company.example", "Brave Work"),
	}, Target("Chrome Personal"))

	decision, err := router.Evaluate("HTTP://company.example/")
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !decision.Matched {
		t.Errorf("matched = %v, want true", decision.Matched)
	}
}

func TestIDNHostsMatchOnlyInIdenticalEncoding(t *testing.T) {
	// Given no IDN conversion is performed, when a Unicode host meets a
	// punycode pattern, it does not match; identical encodings do.
	router := mustRouter(t, []Rule{
		mustRule(t, "unicode", ExactHost, "bücher.example", "Brave Work"),
	}, Target("Chrome Personal"))

	cases := []struct {
		name    string
		url     string
		matched bool
	}{
		{name: "identical unicode form matches", url: "https://bücher.example/", matched: true},
		{name: "punycode form is a different host", url: "https://xn--bcher-kva.example/", matched: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision, err := router.Evaluate(tc.url)
			if err != nil {
				t.Fatalf("Evaluate(%q) failed: %v", tc.url, err)
			}
			if decision.Matched != tc.matched {
				t.Errorf("matched = %v, want %v", decision.Matched, tc.matched)
			}
		})
	}
}

func TestEvaluateIsDeterministic(t *testing.T) {
	// Given the same input twice, when evaluated, decisions are identical
	// including reason ordering.
	router := mustRouter(t, []Rule{
		mustRule(t, "a", Subdomain, "example.com", "Brave Work"),
		mustRule(t, "b", URLRegex, `stable`, "Chrome Work"),
	}, Target("Chrome Personal"))

	first, err := router.Evaluate("https://sub.example.com/stable")
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	second, err := router.Evaluate("https://sub.example.com/stable")
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if !reflect.DeepEqual(first, second) {
		t.Errorf("decisions differ:\nfirst  = %+v\nsecond = %+v", first, second)
	}
}

func TestRouterConstructionValidation(t *testing.T) {
	valid := mustRule(t, "work", ExactHost, "company.example", "Brave Work")

	cases := []struct {
		name     string
		rules    []Rule
		fallback Target
		want     string
	}{
		{name: "empty default target", rules: []Rule{valid}, fallback: "", want: "default target"},
		{name: "rule without name", rules: []Rule{{Name: "", Kind: ExactHost, Pattern: "a.example", Target: "B"}}, fallback: "C", want: "rule name"},
		{name: "duplicate rule names", rules: []Rule{valid, mustRule(t, "work", ExactHost, "b.example", "C")}, fallback: "D", want: "duplicate"},
		{name: "rule without target", rules: []Rule{{Name: "x", Kind: ExactHost, Pattern: "a.example"}}, fallback: "C", want: "target"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewRouter(tc.rules, tc.fallback)
			if err == nil {
				t.Fatalf("NewRouter accepted invalid input, want %q error", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestRuleConstructionValidation(t *testing.T) {
	cases := []struct {
		name    string
		kind    MatcherKind
		pattern string
		want    string
	}{
		{name: "unknown matcher kind", kind: "regex-ish", pattern: "a.example", want: "unknown matcher"},
		{name: "empty host pattern", kind: ExactHost, pattern: "", want: "host pattern"},
		{name: "whitespace in host pattern", kind: Subdomain, pattern: "a b.example", want: "host pattern"},
		{name: "port in host pattern", kind: ExactHost, pattern: "a.example:8080", want: "host pattern"},
		{name: "empty label in host pattern", kind: Subdomain, pattern: "a..example", want: "host pattern"},
		{name: "leading dot in subdomain pattern", kind: Subdomain, pattern: ".example.com", want: "host pattern"},
		{name: "invalid url regex", kind: URLRegex, pattern: "(?<=x)y", want: "invalid url regex"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewRule("rule", tc.kind, tc.pattern, "Brave Work")
			if err == nil {
				t.Fatalf("NewRule(%q, %q) accepted, want %q error", tc.kind, tc.pattern, tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestZeroRuleRouterFallsBack(t *testing.T) {
	// Given no rules and a default, when anything valid is evaluated, the
	// default applies with no reasons.
	router := mustRouter(t, nil, Target("Chrome Personal"))

	decision, err := router.Evaluate("https://anything.example/")
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if decision.Matched || decision.Target != "Chrome Personal" || len(decision.Reasons) != 0 {
		t.Errorf("decision = %+v, want pure fallback", decision)
	}
}

func FuzzEvaluate(f *testing.F) {
	seeds := []string{
		"https://company.example/path?q=1",
		"HTTP://COMPANY.example:8443",
		"https://company.example./",
		"https://sub.company.example/",
		"ftp://company.example",
		"javascript:alert(1)",
		"http://[bad",
		"",
		"https://bücher.example/",
		"https://xn--bcher-kva.example/",
		"http:///path",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	router, err := NewRouter([]Rule{
		mustRule(f, "host", ExactHost, "company.example", "Brave Work"),
		mustRule(f, "sub", Subdomain, "example.com", "Chrome Work"),
		mustRule(f, "re", URLRegex, `(?i)stable`, "Firefox Work"),
	}, Target("default"))
	if err != nil {
		f.Fatalf("router construction failed: %v", err)
	}

	f.Fuzz(func(t *testing.T, rawURL string) {
		decision, err := router.Evaluate(rawURL)
		if err != nil {
			var invalid *InvalidURLError
			if !errors.As(err, &invalid) {
				t.Fatalf("non-URL error for %q: %v", rawURL, err)
			}
			return
		}
		if decision.URL != rawURL {
			t.Fatalf("decision lost the original URL: %q != %q", decision.URL, rawURL)
		}
		if decision.Target == "" {
			t.Fatal("decision has no target")
		}
	})
}

func FuzzNewRule(f *testing.F) {
	f.Add("exact-host", "a.example")
	f.Add("subdomain", ".b.example")
	f.Add("url-regex", "(?<=x)y")
	f.Add("url-regex", "^https://x\\.example/")

	f.Fuzz(func(t *testing.T, kind, pattern string) {
		// Construction must either succeed with a usable rule or return a
		// validation error; it must never panic.
		rule, err := NewRule("fuzz", MatcherKind(kind), pattern, "T")
		if err == nil && rule.Pattern != pattern {
			t.Fatalf("rule pattern %q != input %q", rule.Pattern, pattern)
		}
	})
}
