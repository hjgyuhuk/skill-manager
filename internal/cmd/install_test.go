package cmd

import (
	"testing"
)

func TestParseSource_OwnerRepo(t *testing.T) {
	s := parseSource("vercel-labs/skills")
	if s.cloneURL != "https://github.com/vercel-labs/skills.git" {
		t.Errorf("expected github URL, got %q", s.cloneURL)
	}
	if s.ref != "" {
		t.Errorf("expected empty ref, got %q", s.ref)
	}
}

func TestParseSource_OwnerRepoWithRef(t *testing.T) {
	s := parseSource("vercel-labs/skills@canary")
	if s.cloneURL != "https://github.com/vercel-labs/skills.git" {
		t.Errorf("expected github URL, got %q", s.cloneURL)
	}
	if s.ref != "canary" {
		t.Errorf("expected ref %q, got %q", "canary", s.ref)
	}
}

func TestParseSource_HTTPS(t *testing.T) {
	s := parseSource("https://github.com/org/repo.git")
	if s.cloneURL != "https://github.com/org/repo.git" {
		t.Errorf("expected original URL, got %q", s.cloneURL)
	}
	if s.ref != "" {
		t.Errorf("expected empty ref, got %q", s.ref)
	}
}

func TestParseSource_HTTPSWithRef(t *testing.T) {
	s := parseSource("https://github.com/org/repo@v2")
	if s.cloneURL != "https://github.com/org/repo" {
		t.Errorf("expected URL without ref, got %q", s.cloneURL)
	}
	if s.ref != "v2" {
		t.Errorf("expected ref %q, got %q", "v2", s.ref)
	}
}

func TestParseSource_SSH(t *testing.T) {
	s := parseSource("git@github.com:org/repo.git")
	if s.cloneURL != "git@github.com:org/repo.git" {
		t.Errorf("expected SSH URL, got %q", s.cloneURL)
	}
	if s.ref != "" {
		t.Errorf("expected empty ref, got %q", s.ref)
	}
}

func TestParseSource_SSHNoRefSplit(t *testing.T) {
	// SSH URLs contain @ as part of the identity — we don't split on it.
	// Users should use --ref flag for SSH sources.
	s := parseSource("git@github.com:org/repo.git")
	if s.cloneURL != "git@github.com:org/repo.git" {
		t.Errorf("expected SSH URL, got %q", s.cloneURL)
	}
	if s.ref != "" {
		t.Errorf("expected empty ref, got %q", s.ref)
	}
}
