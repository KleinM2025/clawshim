//go:build !windows

package forward

import (
	"os"
	"os/exec"
)

// The cmd.exe limit this tool works around is Windows-specific; on other
// platforms the shim is still buildable (for tests and development) but the
// extra process-lifetime hardening is a no-op.
func prepareChild(cmd *exec.Cmd) {}

func afterStart(p *os.Process) {}

func cleanupAfterWait() {}
