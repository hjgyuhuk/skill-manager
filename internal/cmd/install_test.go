package cmd

import (
	"os"
	"path/filepath"
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

func TestParseSource_OwnerRepoSubdir(t *testing.T) {
	s := parseSource("mattpocock/skills/grill-me")
	if s.cloneURL != "https://github.com/mattpocock/skills.git" {
		t.Errorf("expected repo URL, got %q", s.cloneURL)
	}
	if s.subdir != "grill-me" {
		t.Errorf("expected subdir %q, got %q", "grill-me", s.subdir)
	}
	if s.ref != "" {
		t.Errorf("expected empty ref, got %q", s.ref)
	}
}

func TestParseSource_OwnerRepoNestedSubdirWithRef(t *testing.T) {
	s := parseSource("mattpocock/skills/typescript/grill-me@canary")
	if s.cloneURL != "https://github.com/mattpocock/skills.git" {
		t.Errorf("expected repo URL, got %q", s.cloneURL)
	}
	if s.subdir != "typescript/grill-me" {
		t.Errorf("expected subdir %q, got %q", "typescript/grill-me", s.subdir)
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

func TestSourceRoot_Subdir(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "skills", "grill-me"), 0755); err != nil {
		t.Fatal(err)
	}

	root, err := sourceRoot(repo, "skills/grill-me")
	if err != nil {
		t.Fatalf("sourceRoot() error: %v", err)
	}
	if root != filepath.Join(repo, "skills", "grill-me") {
		t.Errorf("expected subdir root, got %q", root)
	}
}

func TestSourceRoot_FallsBackToSkillsSubdir(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "skills", "write-a-skill"), 0755); err != nil {
		t.Fatal(err)
	}

	root, err := sourceRoot(repo, "write-a-skill")
	if err != nil {
		t.Fatalf("sourceRoot() error: %v", err)
	}
	if root != filepath.Join(repo, "skills", "write-a-skill") {
		t.Errorf("expected skills subdir root, got %q", root)
	}
}

func TestSourceRoot_RejectsTraversal(t *testing.T) {
	if _, err := sourceRoot(t.TempDir(), "../outside"); err == nil {
		t.Fatal("expected traversal subdir to be rejected")
	}
}
