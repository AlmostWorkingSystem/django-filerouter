//go:build !linux

package supervisor

import "os/exec"

// setPdeathsig is a no-op outside Linux: PR_SET_PDEATHSIG has no portable
// equivalent, so on darwin (local dev builds) an ungracefully-killed parent
// can still orphan its child. Only linux/amd64 and linux/arm64 are
// published release targets, so the released binary always has the guard.
func setPdeathsig(cmd *exec.Cmd) {}
