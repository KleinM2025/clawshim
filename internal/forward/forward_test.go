package forward

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/KleinM2025/clawshim/internal/config"
)

func TestRunExitCodesUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses unix coreutils binaries")
	}
	dir := t.TempDir()
	entry := filepath.Join(dir, "entry.mjs")
	if err := os.WriteFile(entry, []byte("// stub"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{Runtime: "/bin/true", Entry: entry}
	if code := Run(cfg, []string{"a", "b"}); code != 0 {
		t.Fatalf("true exit code = %d, want 0", code)
	}

	cfgFail := &config.Config{Runtime: "/bin/false", Entry: entry}
	if code := Run(cfgFail, nil); code != 1 {
		t.Fatalf("false exit code = %d, want 1", code)
	}
}

func TestRunMissingRuntime(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "entry.mjs")
	if err := os.WriteFile(entry, []byte("// stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Runtime: "definitely-not-a-real-runtime-xyz", Entry: entry}
	if code := Run(cfg, nil); code != 127 {
		t.Fatalf("missing runtime exit code = %d, want 127", code)
	}
}
