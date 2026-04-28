package manager

import (
	"os"
	"path/filepath"
	"testing"
)

func createTestRepo(t *testing.T, structure map[string]string) string {
	t.Helper()
	tmpDir := t.TempDir()
	for relPath, content := range structure {
		full := filepath.Join(tmpDir, relPath)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return tmpDir
}

func TestDiscoverSkills_SKILLMD(t *testing.T) {
	repo := createTestRepo(t, map[string]string{
		"skills/alpha/SKILL.md":  "# Alpha",
		"skills/beta/SKILL.md":   "# Beta",
		"skills/gamma/other.txt": "not a skill",
		"README.md":              "# Repo",
	})

	skills := DiscoverSkills(repo)
	// alpha and beta found via SKILL.md, gamma found via skills/ directory
	if len(skills) != 3 {
		t.Fatalf("expected 3 skills, got %d: %v", len(skills), skills)
	}
	if skills[0].Name != "alpha" || skills[1].Name != "beta" || skills[2].Name != "gamma" {
		t.Errorf("unexpected skills: %v", skills)
	}
}

func TestDiscoverSkills_SKILLMDOnly(t *testing.T) {
	// SKILL.md outside skills/ directory
	repo := createTestRepo(t, map[string]string{
		"alpha/SKILL.md":  "# Alpha",
		"beta/README.md":  "# Beta (no SKILL.md, not under skills/)",
		"README.md":       "# Repo",
	})

	skills := DiscoverSkills(repo)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d: %v", len(skills), skills)
	}
	if skills[0].Name != "alpha" {
		t.Errorf("expected alpha, got %s", skills[0].Name)
	}
}

func TestDiscoverSkills_SkillsDir(t *testing.T) {
	// No SKILL.md files — should still find skills via skills/ directory
	repo := createTestRepo(t, map[string]string{
		"skills/alpha/README.md": "# Alpha",
		"skills/beta/README.md":  "# Beta",
		"README.md":              "# Repo",
	})

	skills := DiscoverSkills(repo)
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d: %v", len(skills), skills)
	}
	if skills[0].Name != "alpha" || skills[1].Name != "beta" {
		t.Errorf("unexpected skills: %v", skills)
	}
}

func TestDiscoverSkills_Combined(t *testing.T) {
	// SKILL.md in beta should take priority over skills/beta
	repo := createTestRepo(t, map[string]string{
		"skills/alpha/README.md": "# Alpha (dir only)",
		"skills/beta/README.md":  "# Beta (dir)",
		"skills/beta/SKILL.md":   "# Beta (skillmd)",
		"gamma/SKILL.md":         "# Gamma (nested SKILL.md)",
	})

	skills := DiscoverSkills(repo)
	if len(skills) != 3 {
		t.Fatalf("expected 3 skills, got %d: %v", len(skills), skills)
	}
	names := make([]string, len(skills))
	for i, s := range skills {
		names[i] = s.Name
	}
	// alpha from skills/ dir, beta from skills/ dir (also has SKILL.md), gamma from SKILL.md
	expected := []string{"alpha", "beta", "gamma"}
	for i, e := range expected {
		if names[i] != e {
			t.Errorf("expected skill[%d] = %q, got %q", i, e, names[i])
		}
	}
}

func TestDiscoverSkills_SkipHidden(t *testing.T) {
	repo := createTestRepo(t, map[string]string{
		"skills/alpha/SKILL.md":    "# Alpha",
		"skills/.hidden/SKILL.md":  "# Hidden",
		".hidden/beta/SKILL.md":    "# Also Hidden",
	})

	skills := DiscoverSkills(repo)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d: %v", len(skills), skills)
	}
	if skills[0].Name != "alpha" {
		t.Errorf("expected alpha, got %s", skills[0].Name)
	}
}

func TestDiscoverSkills_Empty(t *testing.T) {
	repo := createTestRepo(t, map[string]string{
		"README.md": "# Empty repo",
	})

	skills := DiscoverSkills(repo)
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills, got %d", len(skills))
	}
}

func TestDiscoverSkills_Sorted(t *testing.T) {
	repo := createTestRepo(t, map[string]string{
		"skills/zebra/SKILL.md":  "# Z",
		"skills/alpha/SKILL.md":  "# A",
		"skills/middle/SKILL.md": "# M",
	})

	skills := DiscoverSkills(repo)
	if len(skills) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(skills))
	}
	if skills[0].Name != "alpha" || skills[1].Name != "middle" || skills[2].Name != "zebra" {
		t.Errorf("skills not sorted: %v", skills)
	}
}

func TestInstallSkill(t *testing.T) {
	tmpDir, cleanup := setupTestDir(t)
	defer cleanup()
	m := New(tmpDir)

	// Create a source skill directory
	srcDir := t.TempDir()
	skillDir := filepath.Join(srcDir, "my-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# My Skill"), 0644)
	os.WriteFile(filepath.Join(skillDir, "data.txt"), []byte("hello"), 0644)

	if err := m.InstallSkill("my-skill", skillDir, ".agents", nil); err != nil {
		t.Fatalf("InstallSkill() error: %v", err)
	}

	// Verify installed
	dst := filepath.Join(tmpDir, ".agents", "skills", "my-skill")
	if _, err := os.Stat(filepath.Join(dst, "SKILL.md")); err != nil {
		t.Errorf("SKILL.md not installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "data.txt")); err != nil {
		t.Errorf("data.txt not installed: %v", err)
	}
}

func TestInstallSkill_Overwrite(t *testing.T) {
	tmpDir, cleanup := setupTestDir(t)
	defer cleanup()
	m := New(tmpDir)

	// "alpha" already exists from setupTestDir
	srcDir := t.TempDir()
	skillDir := filepath.Join(srcDir, "alpha")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Updated Alpha"), 0644)

	if err := m.InstallSkill("alpha", skillDir, ".agents", nil); err != nil {
		t.Fatalf("InstallSkill() error: %v", err)
	}

	// Verify overwritten
	content, err := os.ReadFile(filepath.Join(tmpDir, ".agents", "skills", "alpha", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "# Updated Alpha" {
		t.Errorf("expected overwritten content, got %q", string(content))
	}
}

func TestInstallSkill_UnknownAgent(t *testing.T) {
	tmpDir, cleanup := setupTestDir(t)
	defer cleanup()
	m := New(tmpDir)

	err := m.InstallSkill("test", t.TempDir(), ".nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

func TestInstallSkill_WithMeta(t *testing.T) {
	tmpDir, cleanup := setupTestDir(t)
	defer cleanup()
	m := New(tmpDir)

	srcDir := t.TempDir()
	skillDir := filepath.Join(srcDir, "my-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# My Skill"), 0644)

	meta := &SkillMeta{
		Source:    "owner/repo",
		CloneURL:  "https://github.com/owner/repo.git",
		SkillName: "my-skill",
		CommitSHA: "abc123",
	}
	if err := m.InstallSkill("my-skill", skillDir, ".agents", meta); err != nil {
		t.Fatalf("InstallSkill() error: %v", err)
	}

	// Verify meta was written
	dst := filepath.Join(tmpDir, ".agents", "skills", "my-skill")
	readMeta, err := ReadMeta(dst)
	if err != nil {
		t.Fatalf("ReadMeta() error: %v", err)
	}
	if readMeta == nil {
		t.Fatal("expected meta, got nil")
	}
	if readMeta.Source != "owner/repo" {
		t.Errorf("expected source %q, got %q", "owner/repo", readMeta.Source)
	}
	if readMeta.CommitSHA != "abc123" {
		t.Errorf("expected commitSHA %q, got %q", "abc123", readMeta.CommitSHA)
	}
	if readMeta.InstalledAt == "" {
		t.Error("expected InstalledAt to be set")
	}
}

func TestWriteReadMeta(t *testing.T) {
	dir := t.TempDir()
	meta := SkillMeta{
		Source:    "vercel-labs/skills",
		CloneURL:  "https://github.com/vercel-labs/skills.git",
		Ref:       "main",
		SkillName: "react",
		CommitSHA: "deadbeef",
	}

	if err := WriteMeta(dir, meta); err != nil {
		t.Fatalf("WriteMeta() error: %v", err)
	}

	read, err := ReadMeta(dir)
	if err != nil {
		t.Fatalf("ReadMeta() error: %v", err)
	}
	if read.Source != meta.Source {
		t.Errorf("source: got %q, want %q", read.Source, meta.Source)
	}
	if read.Ref != meta.Ref {
		t.Errorf("ref: got %q, want %q", read.Ref, meta.Ref)
	}
	if read.CommitSHA != meta.CommitSHA {
		t.Errorf("commitSHA: got %q, want %q", read.CommitSHA, meta.CommitSHA)
	}
}

func TestReadMeta_NotExists(t *testing.T) {
	dir := t.TempDir()
	meta, err := ReadMeta(dir)
	if err != nil {
		t.Fatalf("ReadMeta() error: %v", err)
	}
	if meta != nil {
		t.Errorf("expected nil, got %+v", meta)
	}
}
