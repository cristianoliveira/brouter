package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cristianoliveira/brouter/internal/domain"
	"github.com/cristianoliveira/brouter/internal/infra/config"
	"github.com/cristianoliveira/brouter/internal/infra/luabridge"
)

// decideRoute evaluates one URL with the optional script first and the
// static router second, per the TASK-0025 contract: the script's
// string return wins, everything else (nil defer, unknown target,
// per-URL runtime failure) falls back to the static evaluation. Load-
// level failures return an error and stop the open — no stale script,
// no last-known-good fallback.
//
// A nil *luabridge.Result means "not handled": the static router must
// run. A non-nil Result with Target set means the script decided.
// decideRoute runs the script decision when [lua] is configured and
// reports whether it produced the final decision. handled=false means
// the caller must run the static router (script deferred, an unknown
// target, or a per-URL failure — all reported on stderr as redacted
// categories). Returned errors are visible load failures.
func decideRoute(cfg *config.Config, router *domain.Router, rawURL string, stderr io.Writer) (domain.Decision, bool, error) {
	if cfg.LuaScript == "" {
		return domain.Decision{}, false, nil
	}

	helper, err := resolveLuaHelper()
	if err != nil {
		return domain.Decision{}, false, err
	}

	provider := luabridge.NewProvider(helper, cfg.LuaScript)
	result, err := provider.Decide(context.Background(), rawURL)
	if err != nil {
		return domain.Decision{}, false, err
	}

	switch result.Status {
	case luabridge.StatusOK:
		if _, defined := cfg.Targets[result.Target]; !defined {
			// Fails closed: an unknown target falls back to the static
			// rules with a visible, redacted category (no target name).
			fmt.Fprintln(stderr, "lua route: unknown-target (falling back to config rules)")
			return domain.Decision{}, false, nil
		}
		return domain.Decision{
			Target:      domain.Target(result.Target),
			URL:         rawURL,
			Matched:     true,
			MatchedRule: "lua:route",
		}, true, nil
	case luabridge.StatusError:
		fmt.Fprintf(stderr, "lua route: %s (falling back to config rules)\n", result.Category)
		return domain.Decision{}, false, nil
	default: // StatusDefer
		return domain.Decision{}, false, nil
	}
}

// evaluateStatic is the script-free decision path, kept as a named
// seam so the fallback contract reads explicitly at the call site.
func evaluateStatic(router *domain.Router, rawURL string) (domain.Decision, error) {
	return router.Evaluate(rawURL)
}

// resolveLuaHelper locates the helper beside the brouter executable
// (bundle-relative, like the embedded brouter itself), with an
// environment override for tests.
func resolveLuaHelper() (string, error) {
	if p := os.Getenv("BRROUTER_LUA_HELPER"); p != "" {
		return p, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate lua helper: %w", err)
	}
	candidate := filepath.Join(filepath.Dir(exe), "brouter-lua-helper")
	if _, err := os.Stat(candidate); err != nil {
		return "", fmt.Errorf("lua helper %s is missing: build the bundle or set BRROUTER_LUA_HELPER", candidate)
	}
	return candidate, nil
}
