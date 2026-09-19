//go:build linux

package supervisor

import (
	"os/exec"
	"syscall"
)

// setPdeathsig asks the kernel to SIGKILL the child the instant this
// process dies, for any reason — including ways that skip stopLocked
// entirely (a SIGKILL to us, a terminal SIGHUP on disconnect, a crash).
// Without this, an ungraceful parent death orphans the child (still bound
// to the port), which the next `just dev`'s address ends up colliding with
// on rebind. Graceful shutdown (stopLocked via Restart/Stop) still works
// the same as before; this is purely a safety net for when it doesn't run.
func setPdeathsig(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}
