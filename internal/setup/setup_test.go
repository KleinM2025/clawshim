package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSetupCustomWritesConfig(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "clawshim.json")

	code := Run([]string{
		"--preset", "custom",
		"--runtime", "node",
		"--entry", `C:\somewhere\openclaw.mjs`,
		"--state-dir", `C:\state`,
		"--out", out,
	})
	if code != 0 {
		t.Fatalf("setup exit = %d", code)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if parsed["entry"] != `C:\somewhere\openclaw.mjs` {
		t.Fatalf("entry = %v", parsed["entry"])
	}

	// Second run without --force must refuse.
	if code := Run([]string{
		"--preset", "custom",
		"--runtime", "node",
		"--entry", `C:\somewhere\openclaw.mjs`,
		"--out", out,
	}); code == 0 {
		t.Fatal("expected refusal to overwrite without --force")
	}

	// With --force it must succeed.
	if code := Run([]string{
		"--preset", "custom",
		"--runtime", "node",
		"--entry", `C:\somewhere\openclaw.mjs`,
		"--out", out,
		"--force",
	}); code != 0 {
		t.Fatal("expected --force overwrite to succeed")
	}
}

func TestSetupRequiresPreset(t *testing.T) {
	if code := Run([]string{}); code != 2 {
		t.Fatalf("missing preset should exit 2, got %d", code)
	}
	if code := Run([]string{"--preset", "custom"}); code != 2 {
		t.Fatalf("custom without runtime/entry should exit 2, got %d", code)
	}
	if code := Run([]string{"--preset", "autoclaw"}); code != 2 {
		t.Fatalf("autoclaw without root should exit 2, got %d", code)
	}
}
