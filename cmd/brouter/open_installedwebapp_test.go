package main

import (
	"bytes"
	"testing"

	domainapp "github.com/cristianoliveira/brouter/internal/domain/installedwebapp"
	infraapps "github.com/cristianoliveira/brouter/internal/infra/installedwebapp"
)

type fakeAppDiscovery struct {
	result   infraapps.Result
	launched []string
}

func (f *fakeAppDiscovery) Discover() infraapps.Result { return f.result }

func (f *fakeAppDiscovery) Launch(plan domainapp.LaunchPlan, rawURL string) error {
	f.launched = append(f.launched, plan.Token+"\x00"+rawURL)
	return nil
}

func TestOpenInstalledWebAppWithoutConfiguration(t *testing.T) {
	app, err := domainapp.NewEntry("chatgpt", "https://chatgpt.com/", "https://chatgpt.com/", domainapp.NewLaunchPlan("chatgpt-plan"))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := domainapp.NewCatalog([]domainapp.Entry{app})
	if err != nil {
		t.Fatal(err)
	}
	discovery := &fakeAppDiscovery{result: infraapps.Result{Catalog: catalog}}
	configPath := writeConfig(t, executableTargetConfig("/does/not/need/to/resolve"))
	var stdout, stderr bytes.Buffer

	code := runOpenWithDiscovery(configPath, []string{"https://chatgpt.com/share/abc"}, bytes.NewReader(nil), &stdout, &stderr, discovery)
	if code != exitSuccess {
		t.Fatalf("exit code = %d stderr=%q", code, stderr.String())
	}
	if len(discovery.launched) != 1 || discovery.launched[0] != "chatgpt-plan\x00https://chatgpt.com/share/abc" {
		t.Fatalf("launched = %q", discovery.launched)
	}
	if stdout.String() != "launched: installed web app (chatgpt)\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestOpenStaticRuleWinsOverInstalledWebApp(t *testing.T) {
	app, err := domainapp.NewEntry("chatgpt", "https://chatgpt.com/", "https://chatgpt.com/", domainapp.NewLaunchPlan("chatgpt-plan"))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := domainapp.NewCatalog([]domainapp.Entry{app})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	browser, log := writeFakeBrowser(t, dir, 0)
	configPath := writeConfig(t, executableTargetConfig(browser)+`
[[rules]]
name = "forced"
matcher = "exact-host"
pattern = "chatgpt.com"
target = "fake"
`)
	discovery := &fakeAppDiscovery{result: infraapps.Result{Catalog: catalog}}
	var stdout, stderr bytes.Buffer

	code := runOpenWithDiscovery(configPath, []string{"https://chatgpt.com/share/abc"}, bytes.NewReader(nil), &stdout, &stderr, discovery)
	if code != exitSuccess {
		t.Fatalf("exit code = %d stderr=%q", code, stderr.String())
	}
	if len(discovery.launched) != 0 {
		t.Fatalf("installed app launch = %q, want static rule", discovery.launched)
	}
	if got := readArgvLog(t, log); len(got) != 1 || got[0] != "https://chatgpt.com/share/abc" {
		t.Fatalf("browser argv = %q", got)
	}
}
