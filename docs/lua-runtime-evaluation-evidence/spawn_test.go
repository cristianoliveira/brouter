//go:build ignore

// Reference source only: excluded from the repository build via the
// ignore tag. Copy into a throwaway module with the pinned dependency
// (see RESULTS.md) to re-run. Raw outputs: RESULTS.md.
package eval

import (
	"os/exec"
	"testing"
	"time"
)

// Measures the per-URL cost of a subprocess-isolation provider:
// one fresh runner process per routed URL.
func TestSubprocessSpawnLatency(t *testing.T) {
	start := time.Now()
	n := 100
	for i := 0; i < n; i++ {
		if err := exec.Command("/usr/bin/true").Run(); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("subprocess spawn+exit: %.2f ms/roundtrip (%d iterations)", time.Since(start).Seconds()*1000/float64(n), n)
}
