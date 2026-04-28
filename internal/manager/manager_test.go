package manager

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestDir(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir := t.TempDir()

	// Create .agents/skills and .agents/skills_disabled
	skillsDir := filepath.Join(tmpDir, ".agents", "skills")
	disabledDir := filepath.Join(tmpDir, ".agents", "skills_disabled")

	for _, name := range []string{"alpha", "beta", "gamma", "pixijs-core", "pixijs-math"} {
		dir := filepath.Join(skillsDir, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	for _, name := range []string{"delta", "pixijs-events"} {
		dir := filepath.Join(disabledDir, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create .claude/skills for multi-site testing
	claudeSkills := filepath.Join(tmpDir, ".claude", "skills")
	for _, name := range []string{"alpha", "claude-only"} {
		dir := filepath.Join(claudeSkills, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	return tmpDir, func() { os.RemoveAll(tmpDir) }
}

func newTestManager(t *testing.T) (*Manager, func()) {
	t.Helper()
	tmpDir, cleanup := setupTestDir(t)
	m := New(tmpDir)
	return m, cleanup
}

func TestList(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	result, err := m.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	// Count enabled and disabled
	var enabled, disabled int
	for _, s := range result.Skills {
		if s.Enabled {
			enabled++
		} else {
			disabled++
		}
	}

	// .agents: 5 enabled, 2 disabled; .claude: 2 enabled
	if enabled != 7 {
		t.Errorf("expected 7 enabled, got %d", enabled)
	}
	if disabled != 2 {
		t.Errorf("expected 2 disabled, got %d", disabled)
	}
}

func TestListShowsMultiSite(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	result, err := m.List()
	if err != nil {
		t.Fatal(err)
	}

	// "alpha" should appear twice (from .agents and .claude)
	var alphaCount int
	for _, s := range result.Skills {
		if s.Name == "alpha" {
			alphaCount++
		}
	}
	if alphaCount != 2 {
		t.Errorf("expected alpha in 2 sites, got %d", alphaCount)
	}
}

func TestResolvePattern_Exact(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	matches, err := m.ResolvePattern("gamma")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].Name != "gamma" {
		t.Errorf("expected [gamma], got %v", matches)
	}
}

func TestResolvePattern_Glob(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	matches, err := m.ResolvePattern("pixijs-*")
	if err != nil {
		t.Fatal(err)
	}
	// Should find pixijs-core and pixijs-math (enabled) + pixijs-events (disabled) = 3
	if len(matches) != 3 {
		t.Errorf("expected 3 matches, got %d: %v", len(matches), matches)
	}
}

func TestResolvePattern_MultiSite(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	matches, err := m.ResolvePattern("alpha")
	if err != nil {
		t.Fatal(err)
	}
	// alpha exists in both .agents and .claude
	if len(matches) != 2 {
		t.Errorf("expected 2 matches (multi-site), got %d: %v", len(matches), matches)
	}
	sites := make(map[string]bool)
	for _, match := range matches {
		sites[match.SiteName] = true
	}
	if !sites[".agents"] || !sites[".claude"] {
		t.Errorf("expected matches from .agents and .claude, got %v", sites)
	}
}

func TestDisable(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	matches, _ := m.ResolvePattern("alpha")
	// Filter to only enabled
	var enabled []Match
	for _, match := range matches {
		if match.Site.Name == ".agents" || match.Site.Name == ".claude" {
			src := filepath.Join(match.Site.SkillsDir, match.Name)
			if _, err := os.Stat(src); err == nil {
				enabled = append(enabled, match)
			}
		}
	}

	if err := m.Disable(enabled); err != nil {
		t.Fatalf("Disable() error: %v", err)
	}

	// Verify alpha is gone from both skills dirs
	for _, match := range matches {
		path := filepath.Join(match.Site.SkillsDir, match.Name)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed from %s/skills", match.Name, match.SiteName)
		}
		// Check it's in disabled
		disabledPath := filepath.Join(match.Site.DisabledDir, match.Name)
		if _, err := os.Stat(disabledPath); err != nil {
			t.Errorf("expected %s in %s/skills_disabled", match.Name, match.SiteName)
		}
	}
}

func TestEnable(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	matches, _ := m.ResolvePattern("delta")
	var disabled []Match
	for _, match := range matches {
		src := filepath.Join(match.Site.DisabledDir, match.Name)
		if _, err := os.Stat(src); err == nil {
			disabled = append(disabled, match)
		}
	}

	if err := m.Enable(disabled); err != nil {
		t.Fatalf("Enable() error: %v", err)
	}

	// Check delta is now in skills/
	match := disabled[0]
	if _, err := os.Stat(filepath.Join(match.Site.SkillsDir, "delta")); err != nil {
		t.Error("delta should now be in skills/")
	}
}

func TestUninstall(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	matches, _ := m.ResolvePattern("gamma")
	if err := m.Uninstall(matches); err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}

	match := matches[0]
	if _, err := os.Stat(filepath.Join(match.Site.SkillsDir, "gamma")); !os.IsNotExist(err) {
		t.Error("gamma should be deleted")
	}
}

func TestUninstall_NotFound(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	matches, _ := m.ResolvePattern("nonexistent")
	if len(matches) != 0 {
		t.Error("expected no matches")
	}
}

func TestDisableAndEnableRoundTrip(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	// Find gamma in enabled
	matches, _ := m.ResolvePattern("gamma")
	var enabled []Match
	for _, match := range matches {
		src := filepath.Join(match.Site.SkillsDir, match.Name)
		if _, err := os.Stat(src); err == nil {
			enabled = append(enabled, match)
		}
	}

	// Disable
	if err := m.Disable(enabled); err != nil {
		t.Fatal(err)
	}

	// Verify disabled
	result, _ := m.List()
	found := false
	for _, s := range result.Skills {
		if s.Name == "gamma" && !s.Enabled {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("gamma should be in disabled list")
	}

	// Enable back
	matches, _ = m.ResolvePattern("gamma")
	var disabled []Match
	for _, match := range matches {
		src := filepath.Join(match.Site.DisabledDir, match.Name)
		if _, err := os.Stat(src); err == nil {
			disabled = append(disabled, match)
		}
	}
	if err := m.Enable(disabled); err != nil {
		t.Fatal(err)
	}

	// Verify enabled again
	result, _ = m.List()
	found = false
	for _, s := range result.Skills {
		if s.Name == "gamma" && s.Enabled {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("gamma should be back in enabled list")
	}
}
