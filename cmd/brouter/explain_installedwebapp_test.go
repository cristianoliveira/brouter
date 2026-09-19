package main

import (
	"bytes"
	"testing"

	domainapp "github.com/cristianoliveira/brouter/internal/domain/installedwebapp"
	infraapps "github.com/cristianoliveira/brouter/internal/infra/installedwebapp"
)

func TestExplainUsesInstalledAppDecisionWithoutLaunching(t *testing.T) {
	app, err := domainapp.NewEntry("chatgpt", "https://chatgpt.com/", "https://chatgpt.com/", domainapp.NewLaunchPlan("plan"))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := domainapp.NewCatalog([]domainapp.Entry{app})
	if err != nil {
		t.Fatal(err)
	}
	discovery := &fakeAppDiscovery{result: infraapps.Result{Catalog: catalog}}
	configPath := writeConfig(t, executableTargetConfig("/unused"))
	var stdout, stderr bytes.Buffer

	code := runExplainWithDiscovery(configPath, []string{"https://chatgpt.com/share/abc"}, bytes.NewReader(nil), &stdout, &stderr, discovery)
	if code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`target: installed web app "chatgpt"`)) {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if len(discovery.launched) != 0 {
		t.Fatalf("explain launched app: %q", discovery.launched)
	}
}
