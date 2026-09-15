//go:build ignore

// Reference source only: excluded from the repository build via the
// ignore tag. Copy into a throwaway module with the pinned dependency
// (see RESULTS.md) to re-run. Raw outputs: RESULTS.md.
package eval

import (
	"strings"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func newSafeState() *lua.LState {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	lua.OpenBase(L)
	for _, banned := range []string{"os", "io", "dofile", "loadfile", "require", "load", "loadstring"} {
		L.SetGlobal(banned, lua.LNil)
	}
	// Contract-shaped injection: utils is a plain host-provided global
	// (no package/require machinery at all).
	utils := L.NewTable()
	L.SetField(utils, "epoch", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(1700000000))
		return 1
	}))
	L.SetGlobal("utils", utils)
	return L
}

func TestSandbox(t *testing.T) {
	L := newSafeState()
	defer L.Close()
	for _, probe := range []string{
		"return os", "return io", "return require", "return load", "return dofile",
	} {
		if err := L.DoString(probe); err != nil {
			t.Fatalf("probe %q errored: %v", probe, err)
		}
		if L.Get(-1) != lua.LNil {
			t.Errorf("sandbox leak: %q is accessible", probe)
		}
		L.Pop(1)
	}
	if err := L.DoString("return utils.epoch() == 1700000000"); err != nil {
		t.Fatal(err)
	}
	if !lua.LVAsBool(L.Get(-1)) {
		t.Error("injected clock missing")
	}
	L.Pop(1)
}

func TestTimeout(t *testing.T) {
	L := newSafeState()
	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- L.DoString("while true do end") }()
	<-time.After(200 * time.Millisecond)
	L.Close()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "closed") {
			t.Logf("timeout stop returned err=%v (want closed-state error)", err)
		}
		t.Logf("infinite loop stopped %v after Close()", time.Since(start))
	case <-time.After(2 * time.Second):
		t.Fatal("Close() did not stop the infinite loop within 2s: no hard timeout mechanism")
	}
}

func TestErrorMapping(t *testing.T) {
	L := newSafeState()
	defer L.Close()
	err := L.DoString(`error("boom")`)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected surfaced script error, got %v", err)
	}
}
