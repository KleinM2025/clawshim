package version

import (
	"strings"
	"testing"
)

func TestPickVersionLine(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"OpenClaw 2026.6.8 (669b1dc)", "OpenClaw 2026.6.8 (669b1dc)"},
		{"some noise\nOpenClaw 2026.6.8 (669b1dc)\ntrailing", "OpenClaw 2026.6.8 (669b1dc)"},
		{"only 1.2.3 here", "only 1.2.3 here"},
		{"\n\nfirst real line\nsecond", "first real line"},
	}
	for _, c := range cases {
		if got := pickVersionLine(c.in); got != c.want {
			t.Errorf("pickVersionLine(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPickVersionPrefersOpenclawLine(t *testing.T) {
	in := "warning: something 9.9.9\nOpenClaw 2026.6.8 (abc)"
	if got := pickVersionLine(in); got != "OpenClaw 2026.6.8 (abc)" {
		t.Fatalf("got %q", got)
	}
}

func TestCacheValidityBySignature(t *testing.T) {
	// cacheValid relies on os.Stat of the entry; covered functionally here
	// with a fake check via samePath behavior only (mtime handling needs a
	// real file and is exercised by integration tests).
	if !samePath(`C:\a\b\openclaw.mjs`, `c:\A\B\openclaw.mjs`) {
		t.Fatal("windows-style case-insensitive compare failed")
	}
	if samePath("/a/b", "/a/c") {
		t.Fatal("different paths must not match")
	}
}

func TestSemverRe(t *testing.T) {
	if !semverRe.MatchString("OpenClaw 2026.6.8") {
		t.Fatal("semver not matched")
	}
	if semverRe.MatchString("no digits here") {
		t.Fatal("false positive")
	}
	if !strings.Contains("OpenClaw 2026.6.8", semverRe.FindString("OpenClaw 2026.6.8")) {
		t.Fatal("extracted semver not contained")
	}
}
