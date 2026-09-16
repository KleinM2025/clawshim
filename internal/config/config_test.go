package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpand(t *testing.T) {
	t.Setenv("CLAWSHIM_TEST_VAR", "value1")
	got := Expand(`x %CLAWSHIM_TEST_VAR% y $CLAWSHIM_TEST_VAR z ${CLAWSHIM_TEST_VAR}`)
	want := `x value1 y value1 z value1`
	if got != want {
		t.Fatalf("Expand = %q, want %q", got, want)
	}
	if got := Expand("%CLAWSHIM_TEST_UNSET%"); got != "%CLAWSHIM_TEST_UNSET%" {
		t.Fatalf("unset var should stay literal, got %q", got)
	}
	if got := Expand(""); got != "" {
		t.Fatalf("empty stays empty")
	}
}

func TestLoadFromOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clawshim.json")
	content := `{
  "runtime": "%CLAWSHIM_TEST_RT%",
  "entry": "C:\\somewhere\\openclaw.mjs",
  "env": {"OPENCLAW_STATE_DIR": "%CLAWSHIM_TEST_SD%"}
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAWSHIM_TEST_RT", "node")
	t.Setenv("CLAWSHIM_TEST_SD", `C:\state`)

	cfg := Load(path)
	if cfg.File != path {
		t.Fatalf("File = %q, want %q", cfg.File, path)
	}
	if cfg.Runtime != "node" {
		t.Fatalf("Runtime = %q", cfg.Runtime)
	}
	if cfg.Entry != `C:\somewhere\openclaw.mjs` {
		t.Fatalf("Entry = %q", cfg.Entry)
	}
	if cfg.Env["OPENCLAW_STATE_DIR"] != `C:\state` {
		t.Fatalf("Env not expanded: %q", cfg.Env["OPENCLAW_STATE_DIR"])
	}
	if cfg.LoadNote != "" {
		t.Fatalf("unexpected LoadNote: %q", cfg.LoadNote)
	}
}

func TestLoadMissingExplicit(t *testing.T) {
	cfg := Load(filepath.Join(t.TempDir(), "nope.json"))
	if cfg.LoadNote == "" {
		t.Fatal("expected LoadNote for missing explicit config")
	}
}

func TestValidateRejectsBatchEntry(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "openclaw.cmd")
	if err := os.WriteFile(entry, []byte("@echo off"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Runtime: "node", Entry: entry}
	problems := cfg.Validate()
	found := false
	for _, p := range problems {
		if contains(p, "batch shim") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected batch-shim problem, got %v", problems)
	}
}

func TestValidateSelfLoop(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip("no executable path")
	}
	cfg := &Config{Runtime: "node", Entry: exe}
	problems := cfg.Validate()
	found := false
	for _, p := range problems {
		if contains(p, "infinite loop") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected self-loop problem, got %v", problems)
	}
}

func TestChildEnvInjection(t *testing.T) {
	t.Setenv("CLAWSHIM_TEST_KEEP", "original")
	cfg := &Config{Env: map[string]string{
		"CLAWSHIM_TEST_KEEP": "should-not-override",
		"CLAWSHIM_TEST_NEW":  "added",
	}}
	env := cfg.ChildEnv()
	var keep, new string
	for _, e := range env {
		if len(e) >= len("CLAWSHIM_TEST_KEEP=") && e[:len("CLAWSHIM_TEST_KEEP=")] == "CLAWSHIM_TEST_KEEP=" {
			keep = e
		}
		if len(e) >= len("CLAWSHIM_TEST_NEW=") && e[:len("CLAWSHIM_TEST_NEW=")] == "CLAWSHIM_TEST_NEW=" {
			new = e
		}
	}
	if keep != "CLAWSHIM_TEST_KEEP=original" {
		t.Fatalf("existing env must win, got %q", keep)
	}
	if new != "CLAWSHIM_TEST_NEW=added" {
		t.Fatalf("new env must be injected, got %q", new)
	}
}

func TestVersionCacheToggles(t *testing.T) {
	cfg := &Config{}
	if !cfg.VersionCacheEnabled() {
		t.Fatal("default should be enabled")
	}
	f := false
	cfg.VersionCache = &VersionCache{Enabled: &f}
	if cfg.VersionCacheEnabled() {
		t.Fatal("disabled flag not honored")
	}
	if cfg.VersionCachePath() == "" {
		t.Fatal("default cache path should not be empty")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || index(s, sub) >= 0)
}

func index(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
