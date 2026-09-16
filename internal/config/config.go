// Package config loads and resolves clawshim configuration.
//
// Resolution order (first hit wins):
//
//  1. --shim-config <path> (override passed by the caller)
//  2. CLAWSHIM_CONFIG environment variable
//  3. clawshim.json next to the executable
//  4. %APPDATA%\clawshim\config.json
//  5. built-in autodetection (runtime "node", npm global layout)
//
// Load never fails hard: problems are recorded and surfaced by Validate,
// which callers run before executing anything.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// FileName is the config file base name looked up next to the executable.
	FileName = "clawshim.json"

	envConfig = "CLAWSHIM_CONFIG"
	envDebug  = "CLAWSHIM_DEBUG"
)

// VersionCache configures the cached `--version` answer.
type VersionCache struct {
	Enabled *bool  `json:"enabled,omitempty"`
	Path    string `json:"path,omitempty"`
	MaxAge  string `json:"maxAge,omitempty"` // Go duration, e.g. "720h"
}

// Config is the resolved configuration for one shim invocation.
type Config struct {
	Runtime string            `json:"runtime,omitempty"`
	Entry   string            `json:"entry,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	LogFile string            `json:"logFile,omitempty"`

	VersionCache *VersionCache `json:"versionCache,omitempty"`

	// Source describes where the config came from (for doctor output).
	Source string `json:"-"`
	// File is the config file in use, "" when none.
	File string `json:"-"`
	// LoadNote carries a non-fatal loader problem (missing/unparseable file).
	LoadNote string `json:"-"`
}

// Load resolves configuration. overridePath, when non-empty, is a
// --shim-config value.
func Load(overridePath string) *Config {
	cfg := &Config{}

	type candidate struct {
		path     string
		source   string
		explicit bool
	}
	var candidates []candidate
	if overridePath != "" {
		candidates = append(candidates, candidate{overridePath, "flag --shim-config", true})
	}
	if p := os.Getenv(envConfig); p != "" {
		candidates = append(candidates, candidate{p, "env CLAWSHIM_CONFIG", true})
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, candidate{filepath.Join(filepath.Dir(exe), FileName), "exe directory", false})
	}
	if appdata := os.Getenv("APPDATA"); appdata != "" {
		candidates = append(candidates, candidate{filepath.Join(appdata, "clawshim", "config.json"), `%APPDATA%\clawshim`, false})
	}

	for _, c := range candidates {
		b, err := os.ReadFile(c.path)
		if err != nil {
			if os.IsNotExist(err) {
				if c.explicit {
					cfg.LoadNote = fmt.Sprintf("config file not found: %s", c.path)
					cfg.Source = c.source + " (missing)"
					break
				}
				continue
			}
			cfg.LoadNote = fmt.Sprintf("cannot read %s: %v", c.path, err)
			cfg.Source = c.source
			break
		}
		var parsed Config
		if uerr := json.Unmarshal(b, &parsed); uerr != nil {
			cfg.LoadNote = fmt.Sprintf("cannot parse %s: %v", c.path, uerr)
		} else {
			cfg.Runtime = parsed.Runtime
			cfg.Entry = parsed.Entry
			cfg.Env = parsed.Env
			cfg.LogFile = parsed.LogFile
			cfg.VersionCache = parsed.VersionCache
		}
		cfg.Source = c.source
		cfg.File = c.path
		break
	}
	if cfg.Source == "" {
		cfg.Source = "built-in autodetection"
	}

	// Expand environment references and set defaults.
	cfg.Runtime = Expand(cfg.Runtime)
	cfg.Entry = Expand(cfg.Entry)
	cfg.LogFile = Expand(cfg.LogFile)
	if cfg.VersionCache != nil && cfg.VersionCache.Path != "" {
		cfg.VersionCache.Path = Expand(cfg.VersionCache.Path)
	}
	if len(cfg.Env) > 0 {
		m := make(map[string]string, len(cfg.Env))
		for k, v := range cfg.Env {
			m[k] = Expand(v)
		}
		cfg.Env = m
	}

	if cfg.Runtime == "" {
		cfg.Runtime = "node"
	}
	if cfg.Entry == "" {
		if p, ok := detectEntry(); ok {
			cfg.Entry = p
			cfg.Source += " + autodetected entry"
		}
	}
	if cfg.Entry != "" {
		cfg.Entry = filepath.Clean(cfg.Entry)
	}
	return cfg
}

// detectEntry looks for an npm-installed OpenClaw entrypoint.
func detectEntry() (string, bool) {
	var candidates []string
	if appdata := os.Getenv("APPDATA"); appdata != "" {
		candidates = append(candidates, filepath.Join(appdata, "npm", "node_modules", "openclaw", "openclaw.mjs"))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, "npm", "node_modules", "openclaw", "openclaw.mjs"))
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, true
		}
	}
	return "", false
}

var pctVar = regexp.MustCompile(`%([^%]+)%`)

// Expand expands $VAR, ${VAR} and %VAR% environment references.
func Expand(s string) string {
	if s == "" || (!strings.Contains(s, "%") && !strings.Contains(s, "$")) {
		return s
	}
	s = os.ExpandEnv(s)
	return pctVar.ReplaceAllStringFunc(s, func(m string) string {
		if v, ok := os.LookupEnv(m[1 : len(m)-1]); ok {
			return v
		}
		return m
	})
}

// Validate returns human-readable problems that prevent execution.
func (c *Config) Validate() []string {
	problems := []string{}
	if c.LoadNote != "" {
		problems = append(problems, c.LoadNote)
	}
	if c.Runtime == "" {
		problems = append(problems, `runtime is empty; set "runtime" to "node" or a full path`)
	} else if strings.ContainsAny(c.Runtime, `/\`) {
		if _, err := os.Stat(c.Runtime); err != nil {
			problems = append(problems, fmt.Sprintf("runtime not found: %s", c.Runtime))
		}
	} else if _, err := exec.LookPath(c.Runtime); err != nil {
		problems = append(problems, fmt.Sprintf("runtime %q not found on PATH", c.Runtime))
	}

	if c.Entry == "" {
		problems = append(problems, `no OpenClaw entry found; set "entry" in clawshim.json (e.g. %APPDATA%\npm\node_modules\openclaw\openclaw.mjs)`)
	} else {
		ext := strings.ToLower(filepath.Ext(c.Entry))
		if ext == ".cmd" || ext == ".bat" {
			problems = append(problems, fmt.Sprintf("entry %q is a batch shim; point \"entry\" at openclaw.mjs instead", c.Entry))
		} else if _, err := os.Stat(c.Entry); err != nil {
			problems = append(problems, fmt.Sprintf("entry not found: %s", c.Entry))
		}
		if exe, err := os.Executable(); err == nil && samePath(c.Entry, exe) {
			problems = append(problems, "entry points at clawshim itself (infinite loop guard)")
		}
	}
	return problems
}

// ResolveRuntime returns an executable path for the configured runtime. Bare
// names are resolved through PATH on every call, so installing Node.js later
// (or moving it) needs no config change.
func ResolveRuntime(runtime string) (string, error) {
	if runtime == "" {
		return "", fmt.Errorf("runtime is empty")
	}
	if strings.ContainsAny(runtime, `/\`) {
		st, err := os.Stat(runtime)
		if err != nil || st.IsDir() {
			return "", fmt.Errorf("runtime not found: %s", runtime)
		}
		return runtime, nil
	}
	p, err := exec.LookPath(runtime)
	if err != nil {
		return "", fmt.Errorf("runtime %q not found on PATH (install Node.js or set \"runtime\" to a full path)", runtime)
	}
	return p, nil
}

// ChildEnv returns the environment for the spawned CLI: the current
// environment plus cfg.Env entries that are not already set. Existing values
// always win.
func (c *Config) ChildEnv() []string {
	env := os.Environ()
	for k, v := range c.Env {
		if k == "" {
			continue
		}
		if !hasEnv(env, k) {
			env = append(env, k+"="+v)
		}
	}
	return env
}

func hasEnv(env []string, key string) bool {
	prefix := key + "="
	for _, e := range env {
		if len(e) >= len(prefix) && strings.EqualFold(e[:len(prefix)], prefix) {
			return true
		}
	}
	return false
}

// DefaultVersionCachePath returns %LOCALAPPDATA%\clawshim\version-cache.json
// (os.UserCacheDir elsewhere).
func DefaultVersionCachePath() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "clawshim", "version-cache.json")
	}
	return filepath.Join(os.TempDir(), "clawshim-version-cache.json")
}

// VersionCachePath returns the effective version cache path.
func (c *Config) VersionCachePath() string {
	if c.VersionCache != nil && c.VersionCache.Path != "" {
		return c.VersionCache.Path
	}
	return DefaultVersionCachePath()
}

// VersionCacheEnabled reports whether the version cache is enabled (default true).
func (c *Config) VersionCacheEnabled() bool {
	return c.VersionCache == nil || c.VersionCache.Enabled == nil || *c.VersionCache.Enabled
}

// Logf appends a debug line when logging is enabled (cfg.LogFile, or
// CLAWSHIM_DEBUG=1 which uses the default path). Logging never touches stdout.
func (c *Config) Logf(format string, args ...any) {
	path := c.LogFile
	if path == "" {
		if os.Getenv(envDebug) != "1" {
			return
		}
		path = defaultLogPath()
	}
	line := time.Now().Format(time.RFC3339) + " " + fmt.Sprintf(format, args...) + "\n"
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = f.WriteString(line)
	_ = f.Close()
}

func defaultLogPath() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "clawshim", "debug.log")
	}
	return filepath.Join(os.TempDir(), "clawshim-debug.log")
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
