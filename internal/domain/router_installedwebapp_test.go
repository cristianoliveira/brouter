package domain

import (
	"testing"

	installedwebapp "github.com/cristianoliveira/brouter/internal/domain/installedwebapp"
)

func TestRouterStaticRuleRunsBeforeInstalledApp(t *testing.T) {
	router := mustInstalledRouter(t, []Rule{
		mustRule(t, "forced", ExactHost, "forced.example", "forced-browser"),
	})
	decision, err := router.Evaluate("https://forced.example/")
	if err != nil || decision.Source != "static-rule" || decision.InstalledApp != nil {
		t.Fatalf("decision = %#v, err=%v", decision, err)
	}
}

func TestRouterInstalledAppRunsBeforeConfiguredDefault(t *testing.T) {
	router := mustInstalledRouter(t, nil)
	decision, err := router.Evaluate("https://chatgpt.com/share/1")
	if err != nil || decision.InstalledApp == nil || decision.InstalledApp.ID != "chatgpt" {
		t.Fatalf("decision = %#v, err=%v", decision, err)
	}
}

func TestRouterConfiguredDefaultRemainsLast(t *testing.T) {
	router := mustInstalledRouter(t, nil)
	decision, err := router.Evaluate("https://other.test/")
	if err != nil || decision.InstalledApp != nil || decision.Source != "configured-default" {
		t.Fatalf("decision = %#v, err=%v", decision, err)
	}
}

func TestRouterDoesNotUseInstalledAppsForUserinfoURLs(t *testing.T) {
	router := mustInstalledRouter(t, nil)
	decision, err := router.Evaluate("https://user:secret@example.test/")
	if err != nil || decision.InstalledApp != nil || decision.Source != "configured-default" {
		t.Fatalf("Evaluate() = %#v, err=%v; want configured default without app match", decision, err)
	}
}

func mustInstalledRouter(t *testing.T, rules []Rule) *Router {
	t.Helper()
	app, err := installedwebapp.NewEntry("chatgpt", "https://chatgpt.com/", "https://chatgpt.com/", installedwebapp.NewLaunchPlan("chatgpt"))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := installedwebapp.NewCatalog([]installedwebapp.Entry{app})
	if err != nil {
		t.Fatal(err)
	}
	router, err := NewRouter(rules, "default-browser", catalog)
	if err != nil {
		t.Fatal(err)
	}
	return router
}
