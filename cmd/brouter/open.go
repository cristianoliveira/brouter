package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/cristianoliveira/brouter/internal/domain"
	"github.com/cristianoliveira/brouter/internal/domain/installedwebapp"
	"github.com/cristianoliveira/brouter/internal/infra/config"
	infraapps "github.com/cristianoliveira/brouter/internal/infra/installedwebapp"
	"github.com/cristianoliveira/brouter/internal/infra/launch"
	"github.com/cristianoliveira/brouter/internal/infra/localfile"
	"github.com/cristianoliveira/brouter/internal/infra/routecmd"
	"github.com/cristianoliveira/brouter/internal/infra/routelog"
)

// runOpen implements the body of `brouter open URL`: it evaluates the
// URL with the same domain router as explain, resolves the selected
// target to a real browser executable, and launches it with the URL as
// one structured argument. There is no fallback to the system default
// handler and no silent target switch: every failure is reported and
// stops the open. Flag parsing lives in the Cobra command layer
// (cli.go); args holds zero or one positional URL.
func runOpen(configPath string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return runOpenWithDiscovery(configPath, args, stdin, stdout, stderr, infraapps.System())
}

// appDiscovery is the narrow CLI seam for installed web-app discovery and
// launch. Tests inject fixture-backed adapters; production uses the system
// adapter selected by runtime platform.
type appDiscovery interface {
	Discover() infraapps.Result
	Launch(installedwebapp.LaunchPlan, string) error
}

func runOpenWithDiscovery(configPath string, args []string, stdin io.Reader, stdout, stderr io.Writer, apps appDiscovery) int {
	// A multi-document command line bypasses singleInput (which enforces
	// exactly one URL): the batch is atomic by PREFLIGHT — every
	// document is validated before the first browser starts. That
	// atomicity ends at the first spawn: a browser failure on a later
	// document stops the remaining launches but cannot undo earlier
	// ones (no rollback is possible after a browser spawn).
	if code, handled := openDocumentBatchFromArgs(args, configPath, stderr, stdout); handled {
		return code
	}

	rawURL, ok := singleInput("open", args, stdin, stderr)
	if !ok {
		return exitUsage
	}

	path, cfg, code, ok := loadConfigForOpen(configPath, stderr)
	if !ok {
		return code
	}

	// The opt-in routing log is evidence, not behavior: a nil logger
	// (absent or disabled [log]) costs nothing, and emit failures are
	// redacted warnings that never change routing or exit codes. Local
	// documents bypass routing entirely and stay unlogged.
	logger := routelog.New(cfg.Log, stderr)
	resolveStart := time.Now()

	// Local documents bypass all URL routing (TASK-0033): they go
	// straight to the default configured browser as encoded file URLs.
	// Every other input keeps the exact routing order below.
	if code, handled := launchLocalDocument(cfg, rawURL, stderr, stdout); handled {
		return code
	}

	// Opt-in external routing command: command-first, per plan
	// TASK-0029. Only the exact @default line defers to the static
	// rules and installed-app catalog; every command failure is visible
	// and stops the open — never a silent fallback.
	if code, handled := openByRouteCommand(cfg, path, rawURL, stderr, stdout, logger, resolveStart); handled {
		return code
	}

	appSnapshot := apps.Discover()
	for _, diagnostic := range appSnapshot.Diagnostics {
		fmt.Fprintf(stderr, "brouter: installed-app discovery: %s\n", diagnostic)
	}
	router, err := domain.NewRouter(cfg.Rules, cfg.Default, appSnapshot.Catalog)
	if err != nil {
		fmt.Fprintf(stderr, "config invalid:\n%v\n", err)
		return exitFailure
	}

	decision, err := router.Evaluate(rawURL)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return exitFailure
	}

	if decision.InstalledApp != nil {
		return launchInstalledApp(apps, cfg, decision, rawURL, stderr, stdout, logger, resolveStart)
	}

	return launchConfiguredDecision(cfg, decision, rawURL, stderr, stdout, logger, resolveStart)
}

// launchCommandTarget validates a command decision against the config
// snapshot and launches it. An unknown target is a visible failure —
// it never silently falls back to static rules.
func launchCommandTarget(cfg *config.Config, res routecmd.Result, rawURL string, stderr, stdout io.Writer, logger *routelog.Logger, resolveStart time.Time) int {
	def, defined := cfg.Targets[res.Route]
	if !defined {
		fmt.Fprintf(stderr, "route_command: unknown target (not defined in browsers)\n")
		return exitFailure
	}
	return launchTarget(def, res.Route, rawURL, stderr, stdout, logger, resolveStart)
}

// loadConfigForOpen resolves the config path and loads the config.
// Any failure is already reported; ok is false and code carries the
// process exit status.
func loadConfigForOpen(configPath string, stderr io.Writer) (string, *config.Config, int, bool) {
	path, ok := resolveConfigPath(configPath, stderr)
	if !ok {
		return "", nil, exitFailure, false
	}
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "config invalid:\n%v\n", err)
		return "", nil, exitFailure, false
	}
	return path, cfg, exitSuccess, true
}

func launchConfiguredDecision(cfg *config.Config, decision domain.Decision, rawURL string, stderr, stdout io.Writer, logger *routelog.Logger, resolveStart time.Time) int {
	target, ok := cfg.Targets[string(decision.Target)]
	if !ok {
		// Config validation guarantees defined targets; this is defense
		// in depth, kept visible like every other failure.
		fmt.Fprintf(stderr, "target %q is not defined in browsers\n", decision.Target)
		return exitFailure
	}
	return launchTarget(target, string(decision.Target), rawURL, stderr, stdout, logger, resolveStart)
}

// launchInstalledApp launches a selected app. Only a stale identity causes
// one refresh/re-evaluation; a real process failure is terminal and never
// falls back to a different browser.
func launchInstalledApp(apps appDiscovery, cfg *config.Config, decision domain.Decision, rawURL string, stderr, stdout io.Writer, logger *routelog.Logger, resolveStart time.Time) int {
	entry := decision.InstalledApp
	if entry == nil {
		return exitFailure
	}
	if err := apps.Launch(entry.Launch, rawURL); err != nil {
		if !errors.Is(err, infraapps.ErrStaleLaunchPlan) {
			fmt.Fprintf(stderr, "open failed: %v\n", err)
			logger.Emit(routelog.Entry{
				Target: installedAppLogTarget(entry.ID), Outcome: "launch-failed",
				ResolveMS: time.Since(resolveStart).Milliseconds(),
				LaunchMS:  0, RawURL: rawURL, Error: err.Error(),
			})
			return exitFailure
		}

		// The selected metadata changed between scan and launch. Refresh once,
		// then recompute precedence; stale entries are never launched.
		snapshot := apps.Discover()
		refreshed, refreshErr := domain.NewRouter(cfg.Rules, cfg.Default, snapshot.Catalog)
		if refreshErr != nil {
			fmt.Fprintf(stderr, "open failed: installed-app catalog refresh failed\n")
			return exitFailure
		}
		refreshedDecision, evaluateErr := refreshed.Evaluate(rawURL)
		if evaluateErr != nil {
			fmt.Fprintf(stderr, "%v\n", evaluateErr)
			return exitFailure
		}
		if refreshedDecision.InstalledApp == nil {
			return launchConfiguredDecision(cfg, refreshedDecision, rawURL, stderr, stdout, logger, resolveStart)
		}
		entry = refreshedDecision.InstalledApp
		if err := apps.Launch(entry.Launch, rawURL); err != nil {
			fmt.Fprintf(stderr, "open failed: %v\n", err)
			return exitFailure
		}
	}

	logger.Emit(routelog.Entry{
		Target: installedAppLogTarget(entry.ID), Outcome: "launched",
		ResolveMS: time.Since(resolveStart).Milliseconds(), LaunchMS: 0,
		RawURL: rawURL,
	})
	fmt.Fprintf(stdout, "launched: installed web app (%s)\n", entry.ID)
	return exitSuccess
}

func installedAppLogTarget(id string) string {
	digest := sha256.Sum256([]byte(id))
	return fmt.Sprintf("installed-web-app:%x", digest[:6])
}

// launchTarget resolves and launches one already-validated target,
// then records the open in the opt-in routing log: resolve_ms covers
// the decision (command or static evaluation), launch_ms the browser
// spawn wait. Launch failures are logged with outcome=launch-failed —
// the record documents what happened, it never replaces the visible
// error or the exit status.
func launchTarget(def config.TargetDefinition, name, rawURL string, stderr, stdout io.Writer, logger *routelog.Logger, resolveStart time.Time) int {
	plan, err := launch.Resolve(def, name, launch.SystemEnv())
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return exitFailure
	}

	launchStart := time.Now()
	launchErr := launch.Launch(plan, rawURL, nil)
	if launchErr != nil {
		fmt.Fprintf(stderr, "open failed: %v\n", launchErr)
		logger.Emit(routelog.Entry{
			Target: name, Outcome: "launch-failed",
			ResolveMS: time.Since(resolveStart).Milliseconds(),
			LaunchMS:  time.Since(launchStart).Milliseconds(),
			RawURL:    rawURL, Error: launchErr.Error(),
		})
		return exitFailure
	}
	logger.Emit(routelog.Entry{
		Target: name, Outcome: "launched",
		ResolveMS: time.Since(resolveStart).Milliseconds(),
		LaunchMS:  time.Since(launchStart).Milliseconds(),
		RawURL:    rawURL,
	})

	fmt.Fprintf(stdout, "launched: %s (%s)\n", name, plan.Detail)
	return exitSuccess
}

// openByRouteCommand consults the external routing command and, when
// it decided, launches its target. handled is false only for a
// @default defer, which falls through to the static router.
func openByRouteCommand(cfg *config.Config, configPath, rawURL string, stderr, stdout io.Writer, logger *routelog.Logger, resolveStart time.Time) (int, bool) {
	if cfg.RouteCommand == nil {
		return 0, false
	}
	res, decided, err := decideByCommand(cfg, configPath, rawURL, stderr)
	if err != nil {
		return exitFailure, true
	}
	if !decided {
		return 0, false
	}
	return launchCommandTarget(cfg, res, rawURL, stderr, stdout, logger, resolveStart), true
}

// openDocumentBatchFromArgs runs the preflight-atomic document batch
// when the arguments are one; handled is false for anything else.
// Atomicity covers validation only: all documents pass Resolve before
// any browser starts, but a spawn failure mid-batch leaves already
// launched browsers running — no rollback is possible after a browser
// spawn.
func openDocumentBatchFromArgs(args []string, configPath string, stderr, stdout io.Writer) (int, bool) {
	if !isDocumentBatch(args) {
		return 0, false
	}
	_, cfg, code, ok := loadConfigForOpen(configPath, stderr)
	if !ok {
		return code, true
	}
	return openDocumentBatch(args, cfg, stderr, stdout)
}

// isDocumentBatch reports whether the arguments are a multi-document
// command line: two or more arguments, all local documents. The OS
// never mixes documents with URLs in one event, so a mixed command
// line is not a batch — it fails later as a usage error.
func isDocumentBatch(args []string) bool {
	if len(args) < 2 {
		return false
	}
	for _, arg := range args {
		if !localfile.IsLocalFile(arg) {
			return false
		}
	}
	return true
}

// openDocumentBatch handles a multi-document command line: the whole
// batch is rejected before any browser starts when even one document
// fails preflight — valid files never launch alongside a failed one.
// That is the full guarantee: dispatch afterwards is partial by
// design (see launchDocuments).
func openDocumentBatch(args []string, cfg *config.Config, stderr, stdout io.Writer) (int, bool) {
	fileURLs := make([]string, 0, len(args))
	for _, arg := range args {
		fileURL, err := localfile.Resolve(arg)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return exitFailure, true
		}
		fileURLs = append(fileURLs, fileURL)
	}
	return launchDocuments(cfg, fileURLs, stderr, stdout)
}

// launchDocuments resolves the default configured browser once and
// launches it once per file URL. A launch failure stops the remaining
// launches and is reported visibly — but browsers already spawned
// stay running: no rollback is possible after a browser spawn.
func launchDocuments(cfg *config.Config, fileURLs []string, stderr, stdout io.Writer) (int, bool) {
	name := string(cfg.Default)
	def, defined := cfg.Targets[name]
	if !defined {
		fmt.Fprintf(stderr, "target %q is not defined in browsers\n", name)
		return exitFailure, true
	}

	plan, err := launch.Resolve(def, name, launch.SystemEnv())
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return exitFailure, true
	}
	for _, fileURL := range fileURLs {
		if err := launch.LaunchFile(plan, fileURL, nil); err != nil {
			fmt.Fprintf(stderr, "open failed: %v\n", err)
			return exitFailure, true
		}
		fmt.Fprintf(stdout, "launched: %s (%s)\n", name, plan.Detail)
	}
	return exitSuccess, true
}

// launchLocalDocument opens a validated local document in the default
// configured browser. Local files stay outside both the route_command
// protocol and the static rules (TASK-0033): a document has no URL
// identity to route on, and there is no recursion into the OS default
// handler — the browser is resolved and launched here or the open
// fails visibly with a redacted diagnostic.
func launchLocalDocument(cfg *config.Config, rawInput string, stderr, stdout io.Writer) (int, bool) {
	if !localfile.IsLocalFile(rawInput) {
		return 0, false
	}

	fileURL, err := localfile.Resolve(rawInput)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return exitFailure, true
	}

	name := string(cfg.Default)
	def, defined := cfg.Targets[name]
	if !defined {
		fmt.Fprintf(stderr, "target %q is not defined in browsers\n", name)
		return exitFailure, true
	}

	plan, err := launch.Resolve(def, name, launch.SystemEnv())
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return exitFailure, true
	}
	if err := launch.LaunchFile(plan, fileURL, nil); err != nil {
		fmt.Fprintf(stderr, "open failed: %v\n", err)
		return exitFailure, true
	}

	fmt.Fprintf(stdout, "launched: %s (%s)\n", name, plan.Detail)
	return exitSuccess, true
}

// decideByCommand consults the external routing command. decided is
// true when the command answered with a final target; a null decision
// defers (decided=false, err=nil); any error is terminal and already
// reported on stderr.
func decideByCommand(cfg *config.Config, configPath, rawURL string, stderr io.Writer) (routecmd.Result, bool, error) {
	commander := &routecmd.Commander{Command: cfg.RouteCommand, ConfigPath: configPath}
	res, err := commander.Decide(context.Background(), rawURL)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return routecmd.Result{}, false, err
	}
	return res, !res.Defer, nil
}
