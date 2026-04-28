package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"skillman/internal/manager"

	"github.com/spf13/cobra"
)

type source struct {
	cloneURL string
	ref      string
	subdir   string
}

// parseSource converts user input into a git clone URL and optional ref.
//
// Supported formats:
//
//	owner/repo              → https://github.com/owner/repo.git
//	owner/repo/path         → https://github.com/owner/repo.git, path
//	owner/repo/path@ref     → https://github.com/owner/repo.git, path, ref
//	https://github.com/org/repo[.git][@ref]
//	git@github.com:org/repo[.git][@ref]
func parseSource(input string) source {
	s := source{}

	// Split off @ref suffix — only for owner/repo@ref and https://...@ref.
	// SSH URLs (git@github.com:...) contain @ as part of the identity, not as
	// a ref delimiter, so we skip splitting for those entirely.
	isSSH := strings.HasPrefix(input, "git@") || strings.HasPrefix(input, "ssh://")
	if !isSSH {
		if idx := strings.LastIndex(input, "@"); idx > 0 {
			s.ref = input[idx+1:]
			input = input[:idx]
		}
	}

	// owner/repo shorthand
	if !strings.Contains(input, "://") && !strings.Contains(input, ":") {
		parts := strings.Split(input, "/")
		if len(parts) >= 2 {
			s.cloneURL = fmt.Sprintf("https://github.com/%s/%s.git", parts[0], parts[1])
			if len(parts) > 2 {
				s.subdir = filepath.Join(parts[2:]...)
			}
			return s
		}
		s.cloneURL = fmt.Sprintf("https://github.com/%s.git", input)
		return s
	}

	// Full URL — use as-is
	s.cloneURL = input
	return s
}

// gitClone performs a shallow clone into a temporary directory.
// The caller is responsible for removing the returned directory.
func gitClone(cloneURL, ref string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "skillman-clone-")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, cloneURL, tmpDir)

	cmd := exec.Command("git", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("git clone failed: %w", err)
	}

	return tmpDir, nil
}

func newInstallCmd(m *manager.Manager) *cobra.Command {
	var (
		skillNames []string
		ref        string
		yes        bool
		agent      string
	)

	cmd := &cobra.Command{
		Use:   "install <source>",
		Short: "Install skill(s) from a GitHub repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src := parseSource(args[0])

			// --ref overrides @ref in source
			if ref != "" {
				src.ref = ref
			}

			fmt.Printf("Cloning %s", src.cloneURL)
			if src.ref != "" {
				fmt.Printf(" @ %s", src.ref)
			}
			fmt.Println("...")

			tmpDir, err := gitClone(src.cloneURL, src.ref)
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmpDir)

			// Capture commit SHA for metadata
			commitSHA, _ := manager.GetLocalSHA(tmpDir)

			// Discover skills
			discoverRoot, err := sourceRoot(tmpDir, src.subdir)
			if err != nil {
				return err
			}
			skills := manager.DiscoverSkills(discoverRoot)
			if len(skills) == 0 {
				return fmt.Errorf("no skills found in %s", args[0])
			}

			// Select which skill(s) to install
			var selected []manager.DiscoveredSkill

			if len(skills) == 1 {
				selected = skills
				fmt.Printf("Found skill: %s\n", skills[0].Name)
			} else if len(skillNames) > 0 {
				// --skill flags provided
				nameSet := make(map[string]bool, len(skillNames))
				for _, n := range skillNames {
					nameSet[n] = true
				}
				for _, s := range skills {
					if nameSet[s.Name] {
						selected = append(selected, s)
					}
				}
				if len(selected) == 0 {
					fmt.Println("Available skills:")
					for _, s := range skills {
						fmt.Printf("  - %s\n", s.Name)
					}
					return fmt.Errorf("no matching skills for: %s", strings.Join(skillNames, ", "))
				}
			} else {
				// Interactive selection
				names := make([]string, len(skills))
				for i, s := range skills {
					names[i] = s.Name
				}

				indices, err := multiselect(names)
				if err != nil {
					return fmt.Errorf("selection: %w", err)
				}
				if len(indices) == 0 {
					return fmt.Errorf("no skills selected")
				}
				for _, idx := range indices {
					selected = append(selected, skills[idx])
				}
			}

			// Install each selected skill
			for _, skill := range selected {
				dst := filepath.Join("~", agent, "skills", skill.Name)
				home, _ := os.UserHomeDir()
				dstDisplay := filepath.Join(home, agent, "skills", skill.Name)

				// Check for existing
				if _, err := os.Stat(dstDisplay); err == nil {
					if yes {
						fmt.Printf("Overwriting %s...\n", dst)
					} else {
						fmt.Printf("Skill %q already exists at %s. Overwrite? [y/N] ", skill.Name, dst)
						reader := bufio.NewReader(os.Stdin)
						input, _ := reader.ReadString('\n')
						input = strings.TrimSpace(strings.ToLower(input))
						if input != "y" && input != "yes" {
							fmt.Printf("Skipped: %s\n", skill.Name)
							continue
						}
					}
				}

				meta := &manager.SkillMeta{
					Source:    args[0],
					CloneURL:  src.cloneURL,
					Ref:       src.ref,
					Subdir:    src.subdir,
					SkillName: skill.Name,
					CommitSHA: commitSHA,
				}
				if err := m.InstallSkill(skill.Name, skill.Path, agent, meta); err != nil {
					return fmt.Errorf("install %q: %w", skill.Name, err)
				}
				fmt.Printf("Installed: %s → %s\n", skill.Name, dstDisplay)
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&skillNames, "skill", "s", nil, "Skill name to install (can be repeated)")
	cmd.Flags().StringVarP(&ref, "ref", "r", "", "Git ref (branch/tag) to clone")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation, overwrite existing")
	cmd.Flags().StringVarP(&agent, "agent", "a", ".agents", "Target agent directory name")

	return cmd
}

func sourceRoot(repoDir, subdir string) (string, error) {
	if subdir == "" {
		return repoDir, nil
	}
	clean := filepath.Clean(subdir)
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid source subdir %q", subdir)
	}
	root := filepath.Join(repoDir, clean)
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			skillsRoot := filepath.Join(repoDir, "skills", clean)
			if skillsInfo, skillsErr := os.Stat(skillsRoot); skillsErr == nil && skillsInfo.IsDir() {
				return skillsRoot, nil
			}
		}
		return "", fmt.Errorf("source subdir %q: %w", subdir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("source subdir %q is not a directory", subdir)
	}
	return root, nil
}
