// Package setup implements the --shim-setup config generator.
package setup

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/KleinM2025/clawshim/internal/config"
)

// Run parses setup flags, writes a clawshim.json, and returns an exit code.
func Run(args []string) int {
	fs := flag.NewFlagSet("--shim-setup", flag.ContinueOnError)
	preset := fs.String("preset", "", "config preset: npm | autoclaw | custom")
	autoclawRoot := fs.String("autoclaw-root", "", "AutoClaw installation root (for --preset autoclaw)")
	runtime := fs.String("runtime", "", "runtime executable (for --preset custom)")
	entry := fs.String("entry", "", "path to openclaw.mjs (for --preset custom)")
	stateDir := fs.String("state-dir", "", "value for OPENCLAW_STATE_DIR (optional)")
	out := fs.String("out", "", "output config path (default: next to this executable)")
	force := fs.Bool("force", false, "overwrite an existing config file")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cfg := &config.Config{}
	var warnings []string

	switch *preset {
	case "npm":
		cfg.Runtime = "node"
		cfg.Entry = `%APPDATA%\npm\node_modules\openclaw\openclaw.mjs`
	case "autoclaw":
		if *autoclawRoot == "" {
			fmt.Fprintln(os.Stderr, "clawshim: --preset autoclaw requires --autoclaw-root <dir>")
			return 2
		}
		root := filepath.Clean(*autoclawRoot)
		cfg.Runtime = filepath.Join(root, "resources", "node", "node.exe")
		cfg.Entry = filepath.Join(root, "resources", "gateway", "openclaw", "openclaw.mjs")
		cfg.Env = map[string]string{"OPENCLAW_STATE_DIR": `%USERPROFILE%\.openclaw-autoclaw`}
		// Best-effort existence checks (path layout may drift between releases).
		if _, err := os.Stat(config.Expand(cfg.Runtime)); err != nil {
			warnings = append(warnings, fmt.Sprintf("runtime not found (check --autoclaw-root): %s", cfg.Runtime))
		}
		if _, err := os.Stat(config.Expand(cfg.Entry)); err != nil {
			warnings = append(warnings, fmt.Sprintf("entry not found (check --autoclaw-root): %s", cfg.Entry))
		}
	case "custom":
		if *runtime == "" || *entry == "" {
			fmt.Fprintln(os.Stderr, "clawshim: --preset custom requires --runtime and --entry")
			return 2
		}
		cfg.Runtime, cfg.Entry = *runtime, *entry
		if *stateDir != "" {
			cfg.Env = map[string]string{"OPENCLAW_STATE_DIR": *stateDir}
		}
	default:
		fmt.Fprintln(os.Stderr, "clawshim: choose --preset npm | autoclaw | custom")
		return 2
	}

	target := *out
	if target == "" {
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "clawshim: cannot locate this executable: %v\n", err)
			return 1
		}
		target = filepath.Join(filepath.Dir(exe), config.FileName)
	}
	if _, err := os.Stat(target); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "clawshim: %s already exists (use --force to overwrite)\n", target)
		return 1
	}

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "clawshim: %v\n", err)
		return 1
	}
	b = append(b, '\n')
	if err := os.WriteFile(target, b, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "clawshim: cannot write %s: %v\n", target, err)
		return 1
	}

	fmt.Printf("Wrote config: %s\n", target)
	for _, w := range warnings {
		fmt.Printf("  [warn] %s\n", w)
	}
	fmt.Println("Next steps:")
	fmt.Println("  1. openclaw --shim-doctor")
	fmt.Println("  2. (Multica) multica daemon restart")
	return 0
}
