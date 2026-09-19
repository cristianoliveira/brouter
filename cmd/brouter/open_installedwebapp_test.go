package main

import (
	"bytes"
	"testing"

	domainapp "github.com/cristianoliveira/brouter/internal/domain/installedwebapp"
	infraapps "github.com/cristianoliveira/brouter/internal/infra/installedwebapp"
)

type fakeAppDiscovery struct {
	result      infraapps.Result
	discoveries int
	launched    []string
}

func (f *fakeAppDiscovery) Discover() infraapps.Result {
	f.discoveries++
	return f.result
}

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

func TestOpenExplicitRouteCommandSkipsInstalledAppDiscovery(t *testing.T) {
	dir := t.TempDir()
	browser, log := writeFakeBrowser(t, dir, 0)
	configPath := writeConfig(t, executableTargetConfig(browser)+`
[route_command]
command = ["/bin/sh", "-c", "printf 'fake'"]
`)
	discovery := &fakeAppDiscovery{}
	var stdout, stderr bytes.Buffer

	code := runOpenWithDiscovery(configPath, []string{"https://chatgpt.com/share/abc"}, bytes.NewReader(nil), &stdout, &stderr, discovery)
	if code != exitSuccess || discovery.discoveries != 0 {
		t.Fatalf("code=%d discoveries=%d stderr=%q", code, discovery.discoveries, stderr.String())
	}
	if got := readArgvLog(t, log); len(got) != 1 || got[0] != "https://chatgpt.com/share/abc" {
		t.Fatalf("browser argv = %q", got)
	}
}

func TestOpenDefaultDeferChecksInstalledWebAppBeforeConfiguredDefault(t *testing.T) {
	app, err := domainapp.NewEntry("chatgpt", "https://chatgpt.com/", "https://chatgpt.com/", domainapp.NewLaunchPlan("chatgpt-plan"))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := domainapp.NewCatalog([]domainapp.Entry{app})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	browser, _ := writeFakeBrowser(t, dir, 0)
	configPath := writeConfig(t, executableTargetConfig(browser)+`
[route_command]
command = ["/bin/sh", "-c", "printf '@default'"]
`)
	discovery := &fakeAppDiscovery{result: infraapps.Result{Catalog: catalog}}
	var stdout, stderr bytes.Buffer

	code := runOpenWithDiscovery(configPath, []string{"https://chatgpt.com/share/abc"}, bytes.NewReader(nil), &stdout, &stderr, discovery)
	if code != exitSuccess || len(discovery.launched) != 1 {
		t.Fatalf("code=%d launched=%q stderr=%q", code, discovery.launched, stderr.String())
	}
}

func TestInstalledAppLogTargetIsOpaque(t *testing.T) {
	target := installedAppLogTarget("chatgpt.example.desktop / private path")
	if target == "" || target == "installed-web-app:chatgpt.example.desktop" {
		t.Fatalf("target = %q, want opaque stable identity", target)
	}
	if bytes.Contains([]byte(target), []byte("chatgpt")) || bytes.Contains([]byte(target), []byte("private")) {
		t.Fatalf("target leaks app identity: %q", target)
	}
}

func TestOpenFallsBackToConfiguredDefaultWhenNoAppMatches(t *testing.T) {
	dir := t.TempDir()
	browser, log := writeFakeBrowser(t, dir, 0)
	configPath := writeConfig(t, executableTargetConfig(browser))
	discovery := &fakeAppDiscovery{result: infraapps.Result{Catalog: domainapp.Catalog{}}}
	var stdout, stderr bytes.Buffer

	code := runOpenWithDiscovery(configPath, []string{"https://other.example/"}, bytes.NewReader(nil), &stdout, &stderr, discovery)
	if code != exitSuccess || len(discovery.launched) != 0 {
		t.Fatalf("code=%d launched=%q stderr=%q", code, discovery.launched, stderr.String())
	}
	if got := readArgvLog(t, log); len(got) != 1 || got[0] != "https://other.example/" {
		t.Fatalf("default argv = %q", got)
	}
}
