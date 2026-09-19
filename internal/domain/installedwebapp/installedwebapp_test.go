package installedwebapp

import (
	"net/url"
	"testing"
)

func TestCatalogMatchesSameOriginAndScope(t *testing.T) {
	catalog, err := NewCatalog([]Entry{
		entry(t, "chatgpt", "https://chatgpt.com/", "https://chatgpt.com/"),
		entry(t, "chatgpt-settings", "https://chatgpt.com/", "https://chatgpt.com/settings/"),
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		url  string
		want string
	}{
		{name: "origin", url: "https://chatgpt.com/share/abc", want: "chatgpt"},
		{name: "longest scope", url: "https://chatgpt.com/settings/profile", want: "chatgpt-settings"},
		{name: "scope boundary", url: "https://chatgpt.com/settings-other", want: "chatgpt"},
		{name: "lookalike host", url: "https://chatgpt.com.evil.test/", want: ""},
		{name: "scheme", url: "http://chatgpt.com/", want: ""},
		{name: "port", url: "https://chatgpt.com:444/", want: ""},
		{name: "credentials", url: "https://user:secret@chatgpt.com/", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok, err := catalog.Match(tc.url)
			if tc.name == "credentials" {
				if err == nil {
					t.Fatal("Match() error = nil, want credentials rejection")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				if tc.want != "" {
					t.Fatalf("Match() found no app, want %q", tc.want)
				}
				return
			}
			if got.ID != tc.want {
				t.Fatalf("Match() = %q, want %q", got.ID, tc.want)
			}
		})
	}
}

func TestCatalogTieBreakIsStable(t *testing.T) {
	catalog, err := NewCatalog([]Entry{
		entry(t, "z-app", "https://example.test/", "https://example.test/"),
		entry(t, "a-app", "https://example.test/", "https://example.test/"),
	})
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := catalog.Match("https://example.test/page")
	if err != nil || !ok {
		t.Fatalf("Match() = (%v, %v), want a match", got, ok)
	}
	if got.ID != "a-app" {
		t.Fatalf("Match() ID = %q, want lexical tie-break a-app", got.ID)
	}
}

func TestEntryRejectsMalformedOrUntrustedMetadata(t *testing.T) {
	for _, tc := range []struct {
		name   string
		origin string
		scope  string
	}{
		{name: "non-http origin", origin: "file:///tmp/app", scope: "file:///tmp/"},
		{name: "origin credentials", origin: "https://u:p@example.test/", scope: "https://example.test/"},
		{name: "scope other origin", origin: "https://example.test/", scope: "https://evil.test/"},
		{name: "relative scope", origin: "https://example.test/", scope: "/app"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewEntry("app", tc.origin, tc.scope, NewLaunchPlan("app")); err == nil {
				t.Fatal("NewEntry() error = nil")
			}
		})
	}
}

func entry(t *testing.T, id, origin, scope string) Entry {
	t.Helper()
	entry, err := NewEntry(id, origin, scope, NewLaunchPlan(id))
	if err != nil {
		t.Fatal(err)
	}
	return entry
}

func TestScopeURLIsParsedAndNormalized(t *testing.T) {
	entry, err := NewEntry("app", "HTTPS://Example.TEST:443", "https://example.test/work/", NewLaunchPlan("app"))
	if err != nil {
		t.Fatal(err)
	}
	if entry.Origin.Scheme != "https" || entry.Origin.Host != "example.test" {
		t.Fatalf("Origin = %#v, want normalized origin", entry.Origin)
	}
	if entry.Scope.Path != "/work/" {
		t.Fatalf("Scope.Path = %q, want /work/", entry.Scope.Path)
	}
	if _, err := url.Parse(entry.Scope.String()); err != nil {
		t.Fatal(err)
	}
}
