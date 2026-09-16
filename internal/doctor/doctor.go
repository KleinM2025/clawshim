// Package doctor implements the --shim-doctor diagnostics command.
package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/KleinM2025/clawshim/internal/buildinfo"
	"github.com/KleinM2025/clawshim/internal/config"
	"github.com/KleinM2025/clawshim/internal/version"
)

// Run prints a diagnostic report and returns an exit code (0 = healthy).
func Run(cfg *config.Config) int {
	failed := false

	fmt.Printf("clawshim %s\n", buildinfo.Summary())
	fmt.Printf("\n== Configuration ==\n")
	if exe, err := os.Executable(); err == nil {
		fmt.Printf("  executable: %s\n", exe)
	}
	fmt.Printf("  source:     %s\n", cfg.Source)
	if cfg.File != "" {
		fmt.Printf("  file:       %s\n", cfg.File)
	}
	fmt.Printf("  runtime:    %s\n", cfg.Runtime)
	fmt.Printf("  entry:      %s\n", cfg.Entry)
	if len(cfg.Env) > 0 {
		keys := make([]string, 0, len(cfg.Env))
		for k := range cfg.Env {
			keys = append(keys, k)
		}
		fmt.Printf("  env:        %s\n", strings.Join(keys, ", "))
	}
	if hint := layoutHint(cfg.Entry); hint != "" {
		fmt.Printf("  hint:       %s\n", hint)
	}

	fmt.Printf("\n== Problems ==\n")
	problems := cfg.Validate()
	if len(problems) == 0 {
		fmt.Println("  (none)")
	} else {
		failed = true
		for _, p := range problems {
			fmt.Printf("  [FAIL] %s\n", p)
		}
	}

	fmt.Printf("\n== Live checks ==\n")
	if rt, err := config.ResolveRuntime(cfg.Runtime); err != nil {
		fmt.Printf("  runtime: [FAIL] %v\n", err)
		failed = true
	} else {
		fmt.Printf("  runtime path:    %s\n", rt)
		if out, err := exec.Command(rt, "--version").Output(); err == nil {
			fmt.Printf("  runtime version: %s\n", strings.TrimSpace(string(out)))
		} else {
			fmt.Printf("  runtime version: [warn] %v\n", err)
		}
	}
	if raw, valid := version.CacheState(cfg); raw != "" {
		fmt.Printf("  version cache:   %q (valid=%v)\n  cache file:      %s\n", raw, valid, version.CachePath(cfg))
	} else {
		fmt.Printf("  version cache:   (empty)\n  cache file:      %s\n", version.CachePath(cfg))
	}
	if len(problems) == 0 {
		start := time.Now()
		if raw, err := version.Probe(cfg); err != nil {
			fmt.Printf("  version probe:   [FAIL] %v\n", err)
			failed = true
		} else {
			fmt.Printf("  version probe:   %s (%v)\n", raw, time.Since(start).Round(time.Millisecond))
		}
	} else {
		fmt.Println("  version probe:   (skipped: fix the problems above first)")
	}

	fmt.Printf("\n== Multica integration ==\n")
	fmt.Println("  PATH method:  place this executable's directory before npm's on PATH")
	fmt.Println("  Env method:   set MULTICA_OPENCLAW_PATH to this executable, then:")
	fmt.Println("                multica daemon restart")

	if failed {
		fmt.Println("\nSome checks failed; fix the items above.")
		return 1
	}
	fmt.Println("\nAll checks passed.")
	return 0
}

func layoutHint(entry string) string {
	lower := strings.ToLower(entry)
	switch {
	case entry == "":
		return ""
	case strings.Contains(lower, "autoclaw"):
		return "AutoClaw embedded OpenClaw detected — the primary supported target."
	case strings.Contains(lower, "npm"):
		return "npm-installed OpenClaw detected — supported in design but not the primary tested target; verify with your workload."
	default:
		return "custom OpenClaw layout — not a primarily tested target; verify manually."
	}
}
