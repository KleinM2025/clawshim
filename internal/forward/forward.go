// Package forward launches the configured OpenClaw CLI and proxies stdio.
//
// The child is started with a direct CreateProcess (no cmd.exe): argv,
// stdin/stdout/stderr and the exit code pass through unchanged. This is what
// lifts the effective command-line limit from cmd.exe's 8191 characters to
// CreateProcess's 32767.
package forward

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/KleinM2025/clawshim/internal/config"
)

// Run executes the configured CLI with args and returns the exit code.
func Run(cfg *config.Config, args []string) int {
	if problems := cfg.Validate(); len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintf(os.Stderr, "clawshim: %s\n", p)
		}
		fmt.Fprintf(os.Stderr, "clawshim: run `openclaw --shim-doctor` for details\n")
		return 127
	}
	rt, err := config.ResolveRuntime(cfg.Runtime)
	if err != nil {
		fmt.Fprintf(os.Stderr, "clawshim: %v\n", err)
		return 127
	}

	argv := append([]string{cfg.Entry}, args...)
	cmd := exec.Command(rt, argv...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = cfg.ChildEnv()
	prepareChild(cmd)

	total := 0
	for _, a := range args {
		total += len(a)
	}
	cfg.Logf("forward: args=%d totalLen=%d runtime=%q entry=%q", len(args), total, rt, cfg.Entry)

	start := time.Now()
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "clawshim: failed to start %s: %v\n", rt, err)
		return 127
	}
	afterStart(cmd.Process)
	err = cmd.Wait()
	cleanupAfterWait()

	code := 0
	var ee *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &ee):
		code = ee.ExitCode()
		if code < 0 {
			code = 1
		}
	default:
		fmt.Fprintf(os.Stderr, "clawshim: wait error: %v\n", err)
		code = 127
	}
	cfg.Logf("forward: exit=%d duration=%v", code, time.Since(start).Round(time.Millisecond))
	return code
}
