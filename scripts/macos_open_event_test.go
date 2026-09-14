package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Regression for the TASK-0021 defect: a bundle whose shim only ran a
// bare NSRunLoop never received the GURL Apple Event, so a real
// `/usr/bin/open -a <app> <url>` failed with the LaunchServices
// timeout -1712 and no diagnostics. The shim now initializes
// NSApplication, whose run loop dispatches Apple Events. This test
// exercises the real OS-open path — cold launch and a second event to
// the already-running instance — against a recorder-stub copy of the
// bundle.
//
// Safety: registrations are additive and reverted; cleanup quits the
// test's own app gracefully via AppleScript (no process kills, no
// defaults writes, no user-config or /Applications changes). The test
// app's diagnostics log is redirected into the test workspace via
// `open --env`, so the user's real handler log is never touched.

const lsregBinary = "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"

// openEventEnv is the isolated fixture for one OS-open regression run:
// a signed test bundle with a recorder stub, an isolated handler log,
// and its temporary registration.
type openEventEnv struct {
	work     string
	testApp  string
	bundleID string
	logPath  string
	stubLog  string
}

func newOpenEventEnv(t *testing.T, distBundle string) *openEventEnv {
	t.Helper()
	work := t.TempDir()
	env := &openEventEnv{
		work:    work,
		testApp: filepath.Join(work, "BrouterHandler.app"),
		logPath: filepath.Join(work, "handler.log"),
		stubLog: filepath.Join(work, "receipt.log"),
	}
	if err := os.MkdirAll(filepath.Join(env.testApp, "Contents/MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}

	// A unique bundle identifier keeps this registration separate from
	// any installed handler.
	env.bundleID = fmt.Sprintf("com.cristianoliveira.brouter.ittest%d", time.Now().UnixNano())
	plist := strings.ReplaceAll(
		readFileOrFatal(t, filepath.Join(distBundle, "Contents/Info.plist")),
		"com.cristianoliveira.brouter.handler", env.bundleID)
	if err := os.WriteFile(filepath.Join(env.testApp, "Contents/Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(distBundle, "Contents/MacOS/BrouterHandler"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.testApp, "Contents/MacOS/BrouterHandler"), data, 0o755); err != nil {
		t.Fatal(err)
	}
	installRecorderStub(t, env.work, env.testApp, env.stubLog)

	// Ad-hoc re-sign: the stub replaced sealed content.
	if out, err := exec.Command("codesign", "--force", "--deep", "--sign", "-", env.testApp).CombinedOutput(); err != nil {
		t.Fatalf("re-signing the test bundle failed: %v\n%s", err, out)
	}

	// Register (additive), always reverted at the end.
	if out, err := exec.Command(lsregBinary, "-f", env.testApp).CombinedOutput(); err != nil {
		t.Fatalf("lsregister failed: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		exec.Command(lsregBinary, "-u", env.testApp).Run() //nolint:errcheck — best-effort revert
		os.RemoveAll(env.testApp)
	})
	return env
}

// installRecorderStub swaps the embedded brouter for a compiled
// recorder binary: it is the delivery receiver, writing every
// forwarded argument to receiptLog. A real Mach-O is required — a
// script cannot live inside a signed bundle.
func installRecorderStub(t *testing.T, work, testApp, receiptLog string) {
	t.Helper()
	stubSrc := filepath.Join(work, "recorder.c")
	recorder := "#include <stdio.h>\n" +
		"int main(int argc, char **argv) {\n" +
		"\tFILE *f = fopen(\"" + receiptLog + "\", \"a\");\n" +
		"\tfor (int i = 1; i < argc; i++) fprintf(f, \"RECEIVED %s\\n\", argv[i]);\n" +
		"\tfclose(f);\n" +
		"\treturn 0;\n}\n"
	if err := os.WriteFile(stubSrc, []byte(recorder), 0o644); err != nil {
		t.Fatal(err)
	}
	stubBin := filepath.Join(work, "stub-brouter")
	if out, err := exec.Command("cc", "-o", stubBin, stubSrc).CombinedOutput(); err != nil {
		t.Fatalf("compiling the recorder stub failed: %v\n%s", err, out)
	}
	data, err := os.ReadFile(stubBin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testApp, "Contents/MacOS/brouter"), data, 0o755); err != nil {
		t.Fatal(err)
	}
}

func (e *openEventEnv) openApp(t *testing.T, rawURL string) {
	t.Helper()
	// `open --env` is the only way a LaunchServices-launched process
	// sees our environment: the isolated log keeps assertions off the
	// user's real handler log.
	cmd := exec.Command("open", "--env",
		"BRROUTER_HANDLER_LOG="+e.logPath,
		"-a", e.testApp, rawURL)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("open -a failed for %s: %v\n%s", rawURL, err, out)
	}
}

func (e *openEventEnv) awaitReceipt(t *testing.T, rawURL string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		if data, err := os.ReadFile(e.stubLog); err == nil &&
			strings.Contains(string(data), "RECEIVED "+rawURL) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("stub never received %s within 30s (Apple Event not delivered?)", rawURL)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (e *openEventEnv) quitGracefully(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		out, err := exec.Command("osascript", "-e",
			fmt.Sprintf("tell application id %q to quit", e.bundleID)).CombinedOutput()
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Logf("warning: graceful quit did not confirm within 15s: %s", out)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func TestMacosHandlerReceivesOSOpenEvents(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("LaunchServices open events are macOS-only")
	}
	distBundle := filepath.Join("..", "dist", "BrouterHandler.app")
	if _, err := os.Stat(filepath.Join(distBundle, "Contents/MacOS/BrouterHandler")); err != nil {
		t.Fatalf("bundle not built: %v", err)
	}

	env := newOpenEventEnv(t, distBundle)

	// Cold: the app is not running; `open` starts it and delivers the
	// URL event.
	assertOpenDelivery(t, env, "https://events.example/cold")

	// Already-running: the shim's NSApplication event loop is still
	// alive; the second event reaches the same instance.
	assertOpenDelivery(t, env, "https://events.example/warm")

	assertRedactedLog(t, env, []string{
		"https://events.example/cold",
		"https://events.example/warm",
	})

	env.quitGracefully(t)
}

// assertOpenDelivery runs one OS-open event end to end: `open -a`
// succeeds and the stub receiver records the exact URL.
func assertOpenDelivery(t *testing.T, env *openEventEnv, rawURL string) {
	t.Helper()
	env.openApp(t, rawURL)
	env.awaitReceipt(t, rawURL)
}

// assertRedactedLog verifies the shim's diagnostics: event markers
// present, raw URLs absent.
func assertRedactedLog(t *testing.T, env *openEventEnv, rawURLs []string) {
	t.Helper()
	log := readFileOrFatal(t, env.logPath)
	if !strings.Contains(log, "handler started") || !strings.Contains(log, "forwarding URL event") {
		t.Errorf("handler log = %q, want the redacted event markers", log)
	}
	for _, raw := range rawURLs {
		if strings.Contains(log, raw) {
			t.Errorf("handler log contains the raw URL %q — must stay redacted", raw)
		}
	}
}
