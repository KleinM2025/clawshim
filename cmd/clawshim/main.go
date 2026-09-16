// Command clawshim is a native Windows forwarder for the OpenClaw CLI.
//
// It spawns the real OpenClaw JavaScript entrypoint (node + openclaw.mjs)
// directly through CreateProcess — no cmd.exe anywhere — so command lines
// longer than 8191 characters no longer fail with
// "The command line is too long."
//
// Deploy it as openclaw.exe on PATH, or point MULTICA_OPENCLAW_PATH at it.
// Run `openclaw --shim-info` for the resolved configuration.
//
// Scope: primarily built for the OpenClaw bundled with AutoClaw. Other
// layouts (npm installs, standalone deployments) need your own testing.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/KleinM2025/clawshim/internal/buildinfo"
	"github.com/KleinM2025/clawshim/internal/config"
	"github.com/KleinM2025/clawshim/internal/doctor"
	"github.com/KleinM2025/clawshim/internal/forward"
	"github.com/KleinM2025/clawshim/internal/setup"
	"github.com/KleinM2025/clawshim/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	var configOverride string
	if len(args) > 0 && args[0] == "--shim-config" {
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "clawshim: --shim-config requires a path argument")
			return 2
		}
		configOverride = args[1]
		args = args[2:]
	}

	cfg := config.Load(configOverride)

	if len(args) > 0 {
		switch args[0] {
		case "--shim-info":
			return info(cfg)
		case "--shim-doctor":
			return doctor.Run(cfg)
		case "--shim-setup":
			return setup.Run(args[1:])
		case "--shim-refresh-version":
			return version.Handle(cfg, true)
		case "--shim-help", "--shim":
			usage()
			return 0
		}
	}

	// Version requests are answered from a live probe plus cache.
	if len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
		return version.Handle(cfg, false)
	}

	return forward.Run(cfg, args)
}

func info(cfg *config.Config) int {
	fmt.Printf("clawshim %s\n", buildinfo.Summary())
	if exe, err := os.Executable(); err == nil {
		fmt.Printf("executable:    %s\n", exe)
	}
	fmt.Printf("config from:   %s\n", cfg.Source)
	if cfg.File != "" {
		fmt.Printf("config file:   %s\n", cfg.File)
	}
	fmt.Printf("runtime:       %s\n", cfg.Runtime)
	fmt.Printf("entry:         %s\n", cfg.Entry)
	if cfg.Entry != "" {
		fmt.Printf("entry layout:  %s\n", classify(cfg.Entry))
	}
	if len(cfg.Env) > 0 {
		keys := make([]string, 0, len(cfg.Env))
		for k := range cfg.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Printf("env injected:  %s\n", strings.Join(keys, ", "))
	}
	if raw, valid := version.CacheState(cfg); raw != "" {
		fmt.Printf("version cache: %q (valid=%v)\n", raw, valid)
	} else {
		fmt.Printf("version cache: (empty) — will be filled on first --version\n")
	}
	return 0
}

func classify(entry string) string {
	lower := strings.ToLower(entry)
	switch {
	case strings.Contains(lower, "autoclaw"):
		return "AutoClaw embedded OpenClaw (primary supported target)"
	case strings.Contains(lower, "npm"):
		return "npm-installed OpenClaw (not primary; test your workload)"
	default:
		return "custom layout (not primary; test your workload)"
	}
}

func usage() {
	fmt.Print(`clawshim — native Windows forwarder for the OpenClaw CLI.
Bypasses the cmd.exe 8191-character command-line limit by spawning
node + openclaw.mjs directly via CreateProcess.

Usage:
  openclaw <openclaw args...>       forward to the real CLI (argv/stdio/exit code pass through)
  openclaw --version                answered from a live probe + cache
  openclaw --shim-info              show the resolved configuration
  openclaw --shim-doctor            diagnose configuration and environment
  openclaw --shim-setup [flags]     write a clawshim.json (presets: npm, autoclaw, custom)
  openclaw --shim-refresh-version   force a version re-probe
  openclaw --shim-help              this help

Config resolution:
  --shim-config <path> > CLAWSHIM_CONFIG > ./clawshim.json (next to exe)
  > %APPDATA%\clawshim\config.json > built-in autodetection

Scope: primarily built for the OpenClaw bundled with AutoClaw.
Other layouts need your own testing. See README.md.
`)
}
