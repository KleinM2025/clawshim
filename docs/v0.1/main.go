//go:build ignore

// v0.1 reference — the original single-file forwarder (internal prototype,
// 2026-09-16) that first solved Multica + embedded OpenClaw long-prompt
// failures on one machine. Superseded by the config-driven v0.2+ under
// cmd/clawshim; kept here for provenance.
//
// Personal paths in the original are replaced with placeholders.
//
// Why the approach works: Multica invokes `openclaw agent --message
// <up-to-~30KB prompt>`; a .cmd shim goes through cmd.exe whose command line
// limit is 8191 chars ("The command line is too long."). A native binary
// spawns node.exe directly via CreateProcess (32K hard limit) and passes args
// untouched. It also injects OPENCLAW_STATE_DIR so the embedded openclaw CLI
// uses the AutoClaw state.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	nodeExe     = `D:\AutoClaw\resources\node\node.exe`
	openclawMjs = `D:\AutoClaw\resources\gateway\openclaw\openclaw.mjs`
	stateDir    = `C:\Users\<you>\.openclaw-autoclaw`
)

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		// NOTE: hardcoded in v0.1; v0.2 probes the real version and caches it.
		fmt.Println("openclaw forwarder shim (multica compat) -> OpenClaw 2026.6.8")
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--shim-info" {
		fmt.Printf("node=%s\nmjs=%s\nstate=%s\n", nodeExe, openclawMjs, stateDir)
		return
	}

	cmd := exec.Command(nodeExe, append([]string{openclawMjs}, os.Args[1:]...)...)
	cmd.Dir = "" // inherit cwd
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	// Inject state dir unless caller already set it.
	found := false
	for _, e := range cmd.Env {
		if strings.HasPrefix(e, "OPENCLAW_STATE_DIR=") {
			found = true
			break
		}
	}
	if !found {
		cmd.Env = append(cmd.Env, "OPENCLAW_STATE_DIR="+stateDir)
	}

	err := cmd.Run()
	if err == nil {
		os.Exit(0)
	}
	if ee, ok := err.(*exec.ExitError); ok {
		os.Exit(ee.ExitCode())
	}
	fmt.Fprintf(os.Stderr, "openclaw shim: %v\n", err)
	os.Exit(127)
}
