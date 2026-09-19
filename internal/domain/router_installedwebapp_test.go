package domain

import (
	"testing"

	installedwebapp "github.com/cristianoliveira/brouter/internal/domain/installedwebapp"
)

func TestRouterInstalledAppRunsAfterStaticRules(t *testing.T) {
	app, err := installedwebapp.NewEntry("chatgpt", "https://chatgpt.com/", "https://chatgpt.com/", installedwebapp.NewLaunchPlan("chatgpt"))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := installedwebapp.NewCatalog([]installedwebapp.Entry{app})
	if err != nil {
		t.Fatal(err)
	}
	static := mustRule(t, "forced", ExactHost, "forced.example", "forced-browser")
	router, err := NewRouter([]Rule{static}, "default-browser", catalog)
	if err != nil {
		t.Fatal(err)
	}

	staticDecision, err := router.Evaluate("https://forced.example/")
	if err != nil || staticDecision.Source != "static-rule" || staticDecision.InstalledApp != nil {
		t.Fatalf("static decision = %#v, err=%v", staticDecision, err)
	}
	defaultDecision, err := router.Evaluate("https://other.test/")
	if err != nil {
		t.Fatal(err)
	}
	if defaultDecision.InstalledApp != nil || defaultDecision.Source != "configured-default" {
		t.Fatalf("unmatched decision = %#v", defaultDecision)
	}

	appDecision, err := router.Evaluate("https://chatgpt.com/share/1")
	if err != nil || appDecision.InstalledApp == nil || appDecision.InstalledApp.ID != "chatgpt" {
		t.Fatalf("installed decision = %#v, err=%v", appDecision, err)
	}
}

func TestRouterDoesNotUseInstalledAppsForUserinfoURLs(t *testing.T) {
	app, err := installedwebapp.NewEntry("app", "https://example.test/", "https://example.test/", installedwebapp.NewLaunchPlan("app"))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := installedwebapp.NewCatalog([]installedwebapp.Entry{app})
	if err != nil {
		t.Fatal(err)
	}
	router, err := NewRouter(nil, "default", catalog)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := router.Evaluate("https://user:secret@example.test/")
	if err != nil || decision.InstalledApp != nil || decision.Source != "configured-default" {
		t.Fatalf("Evaluate() = %#v, err=%v; want configured default without app match", decision, err)
	}
}
