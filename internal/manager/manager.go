package manager

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type Site struct {
	Name        string // e.g. ".agents", ".claude"
	SkillsDir   string
	DisabledDir string
}

type Manager struct {
	Sites []Site
}

var DefaultSiteNames = []string{
	".aider-desk",
	".agents",
	".augment",
	".bob",
	".claude",
	".codex",
	".codeartsdoer",
	".codebuddy",
	".codemaker",
	".codestudio",
	".commandcode",
	".continue",
	".cortex",
	".crush",
	".devin",
	".factory",
	".forge",
	".goose",
	".junie",
	".iflow",
	".kilocode",
	".kiro",
	".kode",
	".mcpjam",
	".vibe",
	".mux",
	".openhands",
	".pi",
	".qoder",
	".qwen",
	".rovodev",
	".roo",
	".tabnine/agent",
	".trae",
	".windsurf",
	".zencoder",
	".neovate",
	".pochi",
	".adal",
}

func New(home string) *Manager {
	sites := make([]Site, 0, len(DefaultSiteNames))
	for _, name := range DefaultSiteNames {
		base := filepath.Join(home, name)
		sites = append(sites, Site{
			Name:        name,
			SkillsDir:   filepath.Join(base, "skills"),
			DisabledDir: filepath.Join(base, "skills_disabled"),
		})
	}
	return &Manager{Sites: sites}
}

type SkillInfo struct {
	Name     string
	SiteName string
	Enabled  bool
}

type ListResult struct {
	Skills []SkillInfo
}

func (m *Manager) List() (*ListResult, error) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	result := &ListResult{}

	for _, site := range m.Sites {
		wg.Add(1)
		go func(s Site) {
			defer wg.Done()

			enabled, _ := listDirs(s.SkillsDir)
			disabled, _ := listDirs(s.DisabledDir)

			mu.Lock()
			for _, name := range enabled {
				result.Skills = append(result.Skills, SkillInfo{
					Name:     name,
					SiteName: s.Name,
					Enabled:  true,
				})
			}
			for _, name := range disabled {
				result.Skills = append(result.Skills, SkillInfo{
					Name:     name,
					SiteName: s.Name,
					Enabled:  false,
				})
			}
			mu.Unlock()
		}(site)
	}

	wg.Wait()
	sort.Slice(result.Skills, func(i, j int) bool {
		if result.Skills[i].Name != result.Skills[j].Name {
			return result.Skills[i].Name < result.Skills[j].Name
		}
		return result.Skills[i].SiteName < result.Skills[j].SiteName
	})

	return result, nil
}

type Match struct {
	Name     string
	SiteName string
	Site     Site
}

// ResolvePattern returns matching skills across all sites.
func (m *Manager) ResolvePattern(pattern string) ([]Match, error) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	var all []Match

	for _, site := range m.Sites {
		wg.Add(1)
		go func(s Site) {
			defer wg.Done()

			// Check both enabled and disabled
			for _, dir := range []string{s.SkillsDir, s.DisabledDir} {
				matches, err := ResolvePattern(dir, pattern)
				if err != nil {
					continue
				}
				mu.Lock()
				for _, name := range matches {
					all = append(all, Match{
						Name:     name,
						SiteName: s.Name,
						Site:     s,
					})
				}
				mu.Unlock()
			}
		}(site)
	}

	wg.Wait()

	// Deduplicate by name+site
	seen := make(map[string]struct{})
	var deduped []Match
	for _, m := range all {
		key := m.Name + "\x00" + m.SiteName
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			deduped = append(deduped, m)
		}
	}

	sort.Slice(deduped, func(i, j int) bool {
		if deduped[i].Name != deduped[j].Name {
			return deduped[i].Name < deduped[j].Name
		}
		return deduped[i].SiteName < deduped[j].SiteName
	})

	return deduped, nil
}

// ResolvePattern returns matching directory names from the given dir.
func ResolvePattern(dir string, pattern string) ([]string, error) {
	entries, err := listDirs(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list %s: %w", dir, err)
	}

	var matches []string
	for _, name := range entries {
		matched, err := filepath.Match(pattern, name)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}
		if matched {
			matches = append(matches, name)
		}
	}

	sort.Strings(matches)
	return matches, nil
}

// Disable moves skills from skills/ to skills_disabled/ in their respective sites.
func (m *Manager) Disable(matches []Match) error {
	for _, match := range matches {
		site := match.Site
		if err := ensureDir(site.DisabledDir); err != nil {
			return err
		}

		src := filepath.Join(site.SkillsDir, match.Name)
		dst := filepath.Join(site.DisabledDir, match.Name)

		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("skill %q not found in %s/skills: %w", match.Name, site.Name, err)
		}

		if _, err := os.Stat(dst); err == nil {
			if err := os.RemoveAll(dst); err != nil {
				return fmt.Errorf("remove existing disabled skill %q: %w", match.Name, err)
			}
		}

		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("disable %q: %w", match.Name, err)
		}

		fmt.Printf("Disabled: %s\n", match.Name)
	}

	return nil
}

// Enable moves skills from skills_disabled/ to skills/ in their respective sites.
func (m *Manager) Enable(matches []Match) error {
	for _, match := range matches {
		site := match.Site
		src := filepath.Join(site.DisabledDir, match.Name)
		dst := filepath.Join(site.SkillsDir, match.Name)

		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("skill %q not found in %s/skills_disabled: %w", match.Name, site.Name, err)
		}

		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("enable %q: %w", match.Name, err)
		}

		fmt.Printf("Enabled: %s\n", match.Name)
	}

	return nil
}

// Uninstall deletes skill directories from both skills/ and skills_disabled/.
func (m *Manager) Uninstall(matches []Match) error {
	for _, match := range matches {
		site := match.Site
		path := filepath.Join(site.SkillsDir, match.Name)
		disabledPath := filepath.Join(site.DisabledDir, match.Name)

		removed := false

		if _, err := os.Stat(path); err == nil {
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("uninstall %q from %s/skills: %w", match.Name, site.Name, err)
			}
			fmt.Printf("Removed: %s\n", match.Name)
			removed = true
		}

		if _, err := os.Stat(disabledPath); err == nil {
			if err := os.RemoveAll(disabledPath); err != nil {
				return fmt.Errorf("uninstall %q from %s/skills_disabled: %w", match.Name, site.Name, err)
			}
			fmt.Printf("Removed: %s\n", match.Name)
			removed = true
		}

		if !removed {
			return fmt.Errorf("skill %q not found in %s", match.Name, site.Name)
		}
	}

	return nil
}

func ensureDir(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dir %s: %w", dir, err)
	}
	return nil
}

// listDirs returns sorted directory names (not files) under dir.
func listDirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() && e.Name()[0] != '.' {
			names = append(names, e.Name())
		}
	}

	sort.Strings(names)
	return names, nil
}
