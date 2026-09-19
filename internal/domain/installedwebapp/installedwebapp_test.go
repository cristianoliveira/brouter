package installedwebapp

import "testing"

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
		{name: "scope exact path", url: "https://chatgpt.com/settings", want: "chatgpt-settings"},
		{name: "encoded separator", url: "https://chatgpt.com/settings%2Fprofile", want: "chatgpt"},
		{name: "scope boundary", url: "https://chatgpt.com/settings-other", want: "chatgpt"},
		{name: "lookalike host", url: "https://chatgpt.com.evil.test/", want: ""},
		{name: "scheme", url: "http://chatgpt.com/", want: ""},
		{name: "default port", url: "https://chatgpt.com:443/share/abc", want: "chatgpt"},
		{name: "port", url: "https://chatgpt.com:444/", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) { assertCatalogMatch(t, catalog, tc.url, tc.want) })
	}
}

func assertCatalogMatch(t *testing.T, catalog Catalog, rawURL, want string) {
	t.Helper()
	got, ok, err := catalog.Match(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	if want == "" {
		if ok {
			t.Fatalf("Match() = %q, want no match", got.ID)
		}
		return
	}
	if !ok || got.ID != want {
		t.Fatalf("Match() = (%q, %v), want %q", got.ID, ok, want)
	}
}

func TestCatalogRejectsCredentials(t *testing.T) {
	catalog, err := NewCatalog([]Entry{entry(t, "chatgpt", "https://chatgpt.com/", "https://chatgpt.com/")})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := catalog.Match("https://user:secret@chatgpt.com/"); err == nil {
		t.Fatal("Match() error = nil, want credentials rejection")
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
	if entry.Scope.String() == "" {
		t.Fatal("Scope.String() is empty")
	}
}
