// Package version implements the cached `--version` answer.
//
// The daemon probes `<openclaw> --version` repeatedly (at least once per task
// execution), and a live probe costs a full Node.js start. The cache stores
// the raw version output keyed by the entry file's mtime+size, so upgrading
// OpenClaw invalidates it automatically.
package version

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/KleinM2025/clawshim/internal/config"
)

const probeTimeout = 10 * time.Second

var semverRe = regexp.MustCompile(`\d+\.\d+\.\d+`)

type cache struct {
	Raw        string `json:"raw"`
	EntryPath  string `json:"entryPath"`
	EntryMtime int64  `json:"entryMtimeUnixNano"`
	EntrySize  int64  `json:"entrySize"`
	CheckedAt  int64  `json:"checkedAtUnix"`
}

// Handle answers a version request. force bypasses the cache read.
// Returns the process exit code for the shim.
func Handle(cfg *config.Config, force bool) int {
	if problems := cfg.Validate(); len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintf(os.Stderr, "clawshim: %s\n", p)
		}
		return 127
	}

	var cached *cache
	if !force && cfg.VersionCacheEnabled() {
		cached = readCache(cfg)
		if cached != nil && cacheValid(cached, cfg) {
			fmt.Println(cached.Raw)
			cfg.Logf("version: cache hit raw=%q", cached.Raw)
			return 0
		}
	}

	start := time.Now()
	raw, err := Probe(cfg)
	if err != nil {
		if cached != nil && cached.Raw != "" {
			fmt.Fprintf(os.Stderr, "clawshim: live version probe failed (%v); serving cached value\n", err)
			fmt.Println(cached.Raw)
			return 0
		}
		fmt.Fprintf(os.Stderr, "clawshim: %v\n", err)
		return 1
	}
	cfg.Logf("version: probe ok in %v raw=%q", time.Since(start).Round(time.Millisecond), raw)
	if cfg.VersionCacheEnabled() {
		writeCache(cfg, raw)
	}
	fmt.Println(raw)
	return 0
}

// Probe runs a live `<runtime> <entry> --version` and returns a cleaned
// single-line version string.
func Probe(cfg *config.Config) (string, error) {
	rt, err := config.ResolveRuntime(cfg.Runtime)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, rt, cfg.Entry, "--version")
	cmd.Env = cfg.ChildEnv()
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	runErr := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	errOut := strings.TrimSpace(stderr.String())
	if runErr != nil {
		detail := errOut
		if detail == "" {
			detail = runErr.Error()
		}
		return "", fmt.Errorf("openclaw --version failed: %s", detail)
	}
	if out == "" {
		out = errOut
	}
	if out == "" {
		return "", fmt.Errorf("openclaw --version produced no output")
	}
	return pickVersionLine(out), nil
}

// pickVersionLine prefers a line mentioning OpenClaw and a semver, then any
// semver line, then the first non-empty line.
func pickVersionLine(s string) string {
	var best string
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if best == "" {
			best = l
		}
		if strings.Contains(strings.ToLower(l), "openclaw") && semverRe.MatchString(l) {
			return l
		}
	}
	return best
}

func cacheValid(c *cache, cfg *config.Config) bool {
	if c.Raw == "" || !samePath(c.EntryPath, cfg.Entry) {
		return false
	}
	st, err := os.Stat(cfg.Entry)
	if err != nil {
		return false
	}
	if st.Size() != c.EntrySize || st.ModTime().UnixNano() != c.EntryMtime {
		return false
	}
	if maxAge := maxAgeOf(cfg); maxAge > 0 {
		if time.Since(time.Unix(c.CheckedAt, 0)) > maxAge {
			return false
		}
	}
	return true
}

func maxAgeOf(cfg *config.Config) time.Duration {
	if cfg.VersionCache == nil || cfg.VersionCache.MaxAge == "" {
		return 0
	}
	d, err := time.ParseDuration(cfg.VersionCache.MaxAge)
	if err != nil {
		return 0
	}
	return d
}

func readCache(cfg *config.Config) *cache {
	b, err := os.ReadFile(cfg.VersionCachePath())
	if err != nil {
		return nil
	}
	var c cache
	if json.Unmarshal(b, &c) != nil {
		return nil
	}
	return &c
}

func writeCache(cfg *config.Config, raw string) {
	st, err := os.Stat(cfg.Entry)
	if err != nil {
		return
	}
	c := cache{
		Raw:        raw,
		EntryPath:  cfg.Entry,
		EntryMtime: st.ModTime().UnixNano(),
		EntrySize:  st.Size(),
		CheckedAt:  time.Now().Unix(),
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return
	}
	p := cfg.VersionCachePath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(p, b, 0o644)
}

// CacheState returns the cached raw value and its validity (for doctor).
func CacheState(cfg *config.Config) (string, bool) {
	c := readCache(cfg)
	if c == nil {
		return "", false
	}
	return c.Raw, cacheValid(c, cfg)
}

// CachePath returns the cache file path in use.
func CachePath(cfg *config.Config) string {
	return cfg.VersionCachePath()
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
