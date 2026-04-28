package manager

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxDiscoverDepth = 3
const metaFileName = ".skillman.json"

// SkillMeta is persisted inside each installed skill directory.
// It records the git source so that `update` can pull new versions.
type SkillMeta struct {
	Source      string `json:"source"`   // e.g. "vercel-labs/skills"
	CloneURL    string `json:"cloneURL"` // full git URL
	Ref         string `json:"ref"`      // branch/tag (empty = default)
	Subdir      string `json:"subdir,omitempty"`
	SkillName   string `json:"skillName"`   // name within the source repo
	CommitSHA   string `json:"commitSHA"`   // commit at install time
	InstalledAt string `json:"installedAt"` // ISO 8601 timestamp
}

// WriteMeta writes .skillman.json into a skill directory.
func WriteMeta(skillDir string, meta SkillMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(skillDir, metaFileName), data, 0644)
}

// ReadMeta reads .skillman.json from a skill directory.
// Returns nil if the file does not exist.
func ReadMeta(skillDir string) (*SkillMeta, error) {
	data, err := os.ReadFile(filepath.Join(skillDir, metaFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var meta SkillMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// GetRemoteSHA returns the latest commit SHA on the remote ref
// without downloading any objects. Returns "" if unable to determine.
func GetRemoteSHA(cloneURL, ref string) (string, error) {
	args := []string{"ls-remote"}
	if ref != "" {
		// --refs is safe for explicit branches/tags
		args = append(args, "--refs", cloneURL, "refs/heads/"+ref, "refs/tags/"+ref)
	} else {
		// HEAD is a pseudo-ref; --refs would exclude it
		args = append(args, cloneURL, "HEAD")
	}

	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", fmt.Errorf("git ls-remote: %w", err)
	}

	// Output format: "<sha>\t<ref>"
	line := strings.TrimSpace(string(out))
	if line == "" {
		return "", nil
	}
	parts := strings.SplitN(line, "\t", 2)
	if len(parts) < 1 {
		return "", nil
	}
	return parts[0], nil
}

// GetLocalSHA returns the HEAD commit SHA of a local git repo.
func GetLocalSHA(repoDir string) (string, error) {
	out, err := exec.Command("git", "-C", repoDir, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// InstalledSkill represents a skill that has .skillman.json metadata.
type InstalledSkill struct {
	Name     string
	SiteName string
	Dir      string // full path to skill directory
	Meta     SkillMeta
}

// ScanInstalled returns all skills across all sites that were installed via
// skillman (have a .skillman.json file).
func (m *Manager) ScanInstalled(nameFilter string) []InstalledSkill {
	var mu sync.Mutex
	var wg sync.WaitGroup
	var result []InstalledSkill

	for _, site := range m.Sites {
		wg.Add(1)
		go func(s Site) {
			defer wg.Done()

			for _, dir := range []string{s.SkillsDir, s.DisabledDir} {
				entries, err := os.ReadDir(dir)
				if err != nil {
					continue
				}
				for _, e := range entries {
					if !e.IsDir() || e.Name()[0] == '.' {
						continue
					}
					if nameFilter != "" && e.Name() != nameFilter {
						continue
					}
					skillDir := filepath.Join(dir, e.Name())
					meta, err := ReadMeta(skillDir)
					if err != nil || meta == nil {
						continue
					}
					mu.Lock()
					result = append(result, InstalledSkill{
						Name:     e.Name(),
						SiteName: s.Name,
						Dir:      skillDir,
						Meta:     *meta,
					})
					mu.Unlock()
				}
			}
		}(site)
	}

	wg.Wait()

	sort.Slice(result, func(i, j int) bool {
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		return result[i].SiteName < result[j].SiteName
	})

	return result
}

// DiscoveredSkill represents a skill found in a repository.
type DiscoveredSkill struct {
	Name string // directory name used as skill identifier
	Path string // absolute path in the cloned repo
}

// DiscoverSkills scans repoDir for skills using two strategies:
//  1. Directories containing a SKILL.md file (recursive, max depth 3)
//  2. Subdirectories under a top-level skills/ directory
//
// Results are deduplicated by name, with SKILL.md matches taking priority.
func DiscoverSkills(repoDir string) []DiscoveredSkill {
	byName := make(map[string]DiscoveredSkill)

	// Strategy 2: skills/ directory (lower priority, discovered first so SKILL.md overwrites)
	skillsDir := filepath.Join(repoDir, "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() && e.Name()[0] != '.' {
				name := e.Name()
				if _, exists := byName[name]; !exists {
					byName[name] = DiscoveredSkill{
						Name: name,
						Path: filepath.Join(skillsDir, name),
					}
				}
			}
		}
	}

	// Strategy 1: SKILL.md detection (higher priority, overwrites strategy 2)
	filepath.WalkDir(repoDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		// Respect max depth
		rel, _ := filepath.Rel(repoDir, path)
		depth := 0
		if rel != "." {
			for _, c := range rel {
				if c == filepath.Separator {
					depth++
				}
			}
			depth++
		}
		if depth > maxDiscoverDepth {
			return filepath.SkipDir
		}
		// Skip hidden directories
		if d.Name()[0] == '.' && path != repoDir {
			return filepath.SkipDir
		}

		skillMD := filepath.Join(path, "SKILL.md")
		if _, err := os.Stat(skillMD); err == nil {
			name := d.Name()
			byName[name] = DiscoveredSkill{
				Name: name,
				Path: path,
			}
		}
		return nil
	})

	// Convert to sorted slice
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]DiscoveredSkill, 0, len(names))
	for _, name := range names {
		result = append(result, byName[name])
	}
	return result
}

// InstallSkill copies the skill directory at srcPath into the target agent's
// skills directory (~/{siteName}/skills/{skillName}/).
// If meta is non-nil, it is written as .skillman.json inside the skill directory.
// Uses a temp dir + atomic rename so a failed copy never destroys the existing install.
func (m *Manager) InstallSkill(skillName, srcPath, siteName string, meta *SkillMeta) error {
	// Find the target site
	var target *Site
	for i := range m.Sites {
		if m.Sites[i].Name == siteName {
			target = &m.Sites[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("unknown agent %q", siteName)
	}

	// Ensure skills directory exists
	if err := os.MkdirAll(target.SkillsDir, 0755); err != nil {
		return fmt.Errorf("create skills dir: %w", err)
	}

	dst := filepath.Join(target.SkillsDir, skillName)

	// Copy to a temp dir in the same parent (so rename is atomic)
	tmpDst, err := os.MkdirTemp(target.SkillsDir, ".skillman-tmp-")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	success := false
	defer func() {
		if !success {
			os.RemoveAll(tmpDst)
		}
	}()

	if err := CopyDir(srcPath, tmpDst); err != nil {
		return fmt.Errorf("copy skill %q: %w", skillName, err)
	}

	// Write metadata
	if meta != nil {
		meta.InstalledAt = time.Now().UTC().Format(time.RFC3339)
		if err := WriteMeta(tmpDst, *meta); err != nil {
			return fmt.Errorf("write meta for %q: %w", skillName, err)
		}
	}

	// Atomic replace: move old to backup, move new in, clean up backup
	if err := ReplaceDir(tmpDst, dst); err != nil {
		return fmt.Errorf("install %q: %w", skillName, err)
	}

	success = true
	return nil
}

// CopyDir recursively copies src to dst, preserving file permissions.
func CopyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		return CopyFile(path, target)
	})
}

// CopyFile copies a single file, preserving permissions.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// ReplaceDir atomically replaces dst with src using a backup strategy:
//  1. If dst exists, rename it to a unique temporary sibling
//  2. Rename src to dst
//  3. On success, remove backup; on failure, restore backup
//
// This ensures dst is never left in a missing/broken state.
func ReplaceDir(src, dst string) error {
	dstExists := false
	var backup string

	if _, err := os.Stat(dst); err == nil {
		dstExists = true
		parent := filepath.Dir(dst)
		// Create a unique backup path (remove the empty dir, use the name for rename)
		tmpBackup, err := os.MkdirTemp(parent, ".skillman-backup-")
		if err != nil {
			return fmt.Errorf("create backup path: %w", err)
		}
		if err := os.Remove(tmpBackup); err != nil {
			return fmt.Errorf("prepare backup path: %w", err)
		}
		backup = tmpBackup

		if err := os.Rename(dst, backup); err != nil {
			return fmt.Errorf("backup old dir: %w", err)
		}
	}

	// Move new into place
	if err := os.Rename(src, dst); err != nil {
		if dstExists {
			if restoreErr := os.Rename(backup, dst); restoreErr != nil {
				return fmt.Errorf("rename new dir: %w (also failed to restore backup: %v)", err, restoreErr)
			}
		}
		return fmt.Errorf("rename new dir: %w", err)
	}

	// Success — clean up backup
	if dstExists {
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("replace succeeded but failed to clean up backup %s: %w", backup, err)
		}
	}
	return nil
}
