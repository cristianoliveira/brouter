package routecmd

import (
	"os"
	"sync"
)

// killGate synchronizes kill requests with process publication.
//
// A writer's overflow can fire kill() before Start has published the
// process; the request is recorded as pending and is executed the
// moment publish() receives the process — so an instant-overflowing
// child is still killed immediately, deterministically, with no
// race between publication and termination.
type killGate struct {
	mu      sync.Mutex
	proc    *os.Process
	pending bool
}

// kill requests termination. Before publication it is recorded as
// pending; after publication it terminates the process group at once.
func (g *killGate) kill() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.proc == nil {
		g.pending = true
		return
	}
	killProcess(g.proc)
}

// publish hands the started process to the gate and drains any kill
// request that arrived in the pre-publication window.
func (g *killGate) publish(p *os.Process) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.proc = p
	if g.pending {
		killProcess(p)
	}
}
