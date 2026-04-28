package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillman/internal/manager"

	"github.com/spf13/cobra"
)

type sourceKey struct {
	cloneURL string
	ref      string
	subdir   string
}

type clonedSource struct {
	tmpDir    string
	skills    []manager.DiscoveredSkill
	commitSHA string
}

type updateItem struct {
	skill     manager.InstalledSkill
	remoteSHA string
	tmpDir    string
	skills    []manager.DiscoveredSkill
}

func newUpdateCmd(m *manager.Manager) *cobra.Command {
	var (
		yes   bool
		agent string
	)

	cmd := &cobra.Command{
		Use:   "update [name]",
		Short: "Update skill(s) installed via skillman",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nameFilter := ""
			if len(args) > 0 {
				nameFilter = args[0]
			}

			// Find all installed skills with metadata
			installed := m.ScanInstalled(nameFilter)
			if len(installed) == 0 {
				if nameFilter != "" {
					return fmt.Errorf("skill %q was not installed via skillman (no .skillman.json)", nameFilter)
				}
				return fmt.Errorf("no skills installed via skillman found")
			}

			// Filter by agent if specified
			if agent != "" {
				var filtered []manager.InstalledSkill
				for _, s := range installed {
					if s.SiteName == agent {
						filtered = append(filtered, s)
					}
				}
				if len(filtered) == 0 {
					return fmt.Errorf("no installed skills found in %s", agent)
				}
				installed = filtered
			}

			// Group by source to avoid duplicate ls-remote calls
			sources := make(map[sourceKey]string) // key → remote SHA
			for _, s := range installed {
				k := sourceKey{s.Meta.CloneURL, s.Meta.Ref, s.Meta.Subdir}
				if _, checked := sources[k]; !checked {
					fmt.Printf("Checking %s", s.Meta.Source)
					if s.Meta.Ref != "" {
						fmt.Printf(" @ %s", s.Meta.Ref)
					}
					fmt.Print("...")
					sha, err := manager.GetRemoteSHA(s.Meta.CloneURL, s.Meta.Ref)
					if err != nil {
						fmt.Printf(" error: %v\n", err)
						sources[k] = "" // unable to determine
					} else {
						fmt.Printf(" %s\n", sha[:min(len(sha), 8)])
						sources[k] = sha
					}
				}
			}

			// Determine which skills need updates
			var toUpdate []updateItem
			var upToDate []string

			for _, s := range installed {
				k := sourceKey{s.Meta.CloneURL, s.Meta.Ref, s.Meta.Subdir}
				remoteSHA := sources[k]
				if remoteSHA == "" || remoteSHA == s.Meta.CommitSHA {
					upToDate = append(upToDate, s.Name)
					continue
				}
				toUpdate = append(toUpdate, updateItem{skill: s, remoteSHA: remoteSHA})
			}

			if len(upToDate) > 0 {
				for _, name := range upToDate {
					fmt.Printf("  %s: up to date\n", name)
				}
			}

			if len(toUpdate) == 0 {
				fmt.Println("\nAll skills are up to date.")
				return nil
			}

			// Confirm before updating
			if !yes {
				fmt.Printf("\nUpdate %d skill(s):\n", len(toUpdate))
				for _, item := range toUpdate {
					shortOld := item.skill.Meta.CommitSHA
					if len(shortOld) > 8 {
						shortOld = shortOld[:8]
					}
					shortNew := item.remoteSHA
					if len(shortNew) > 8 {
						shortNew = shortNew[:8]
					}
					fmt.Printf("  %s (%s → %s)\n", item.skill.Name, shortOld, shortNew)
				}
				fmt.Print("\nProceed? [y/N] ")
				reader := bufio.NewReader(os.Stdin)
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(strings.ToLower(input))
				if input != "y" && input != "yes" {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			// Clone and discover skills for each unique source
			cloned := make(map[sourceKey]*clonedSource)

			for i := range toUpdate {
				item := &toUpdate[i]
				k := sourceKey{item.skill.Meta.CloneURL, item.skill.Meta.Ref, item.skill.Meta.Subdir}

				cs, ok := cloned[k]
				if !ok {
					fmt.Printf("Cloning %s", item.skill.Meta.Source)
					if item.skill.Meta.Ref != "" {
						fmt.Printf(" @ %s", item.skill.Meta.Ref)
					}
					fmt.Println("...")

					tmpDir, err := gitClone(item.skill.Meta.CloneURL, item.skill.Meta.Ref)
					if err != nil {
						return err
					}
					defer os.RemoveAll(tmpDir)

					commitSHA, _ := manager.GetLocalSHA(tmpDir)
					discoverRoot, err := sourceRoot(tmpDir, item.skill.Meta.Subdir)
					if err != nil {
						return err
					}
					skills := manager.DiscoverSkills(discoverRoot)

					cs = &clonedSource{tmpDir: tmpDir, skills: skills, commitSHA: commitSHA}
					cloned[k] = cs
				}

				item.tmpDir = cs.tmpDir
				item.skills = cs.skills
			}

			// Perform updates
			updated := 0
			for _, item := range toUpdate {
				if err := updateOneSkill(item, cloned); err != nil {
					fmt.Printf("  %s: %v\n", item.skill.Name, err)
					continue
				}
				fmt.Printf("  %s: updated\n", item.skill.Name)
				updated++
			}

			fmt.Printf("\nUpdated %d skill(s).\n", updated)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation")
	cmd.Flags().StringVarP(&agent, "agent", "a", "", "Filter by agent directory name")

	return cmd
}

// updateOneSkill handles the update of a single skill. The defer inside ensures
// any temp directory is cleaned up as soon as this function returns.
func updateOneSkill(item updateItem, cloned map[sourceKey]*clonedSource) error {
	// Find the matching skill in the cloned repo
	var found *manager.DiscoveredSkill
	for _, s := range item.skills {
		if s.Name == item.skill.Meta.SkillName {
			found = &s
			break
		}
	}
	if found == nil {
		return fmt.Errorf("skill %q not found in source (skipped)", item.skill.Meta.SkillName)
	}

	// Copy to temp dir in same parent (for atomic rename)
	parentDir := filepath.Dir(item.skill.Dir)
	tmpDst, err := os.MkdirTemp(parentDir, ".skillman-tmp-")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() {
		// Clean up temp dir if it still exists (ReplaceDir renames it away on success)
		if _, err := os.Stat(tmpDst); err == nil {
			os.RemoveAll(tmpDst)
		}
	}()

	if err := manager.CopyDir(found.Path, tmpDst); err != nil {
		return fmt.Errorf("copy failed: %w", err)
	}

	// Write metadata with updated SHA
	meta := item.skill.Meta
	cs := cloned[sourceKey{item.skill.Meta.CloneURL, item.skill.Meta.Ref, item.skill.Meta.Subdir}]
	if cs == nil {
		return fmt.Errorf("source clone not found")
	}
	meta.CommitSHA = cs.commitSHA
	if err := manager.WriteMeta(tmpDst, meta); err != nil {
		return fmt.Errorf("write meta failed: %w", err)
	}

	// Atomic replace: backup old, move new in
	if err := manager.ReplaceDir(tmpDst, item.skill.Dir); err != nil {
		return fmt.Errorf("replace failed: %w", err)
	}

	return nil
}
