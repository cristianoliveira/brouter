package routecmd

import (
	"os/exec"
	"testing"
	"time"
)

// Deterministic pre-publication kill: a kill() recorded before
// publish() executes the instant the process is published — the
// pending process is dead when publish returns.
func TestKillGatePendingKillFiresOnPublish(t *testing.T) {
	g := &killGate{}
	g.kill() // pending: no process published yet

	cmd := exec.Command("sleep", "30")
	cmd.SysProcAttr = groupAttr() // the runner starts children as group leaders
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start sleep: %v", err)
	}
	g.publish(cmd.Process)

	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
		// killed promptly on publication
	case <-time.After(2 * time.Second):
		t.Fatal("pending kill did not fire on publication")
	}
}

// Post-publication kills fire immediately as well.
func TestKillGateKillAfterPublish(t *testing.T) {
	g := &killGate{}
	cmd := exec.Command("sleep", "30")
	cmd.SysProcAttr = groupAttr()
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start sleep: %v", err)
	}
	g.publish(cmd.Process)
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	g.kill()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("post-publication kill did not fire")
	}
}
