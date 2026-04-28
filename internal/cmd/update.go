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
			type sourceKey struct {
				cloneURL string
				ref      string
			}
			sources := make(map[sourceKey]string) // key → remote SHA
			for _, s := range installed {
				k := sourceKey{s.Meta.CloneURL, s.Meta.Ref}
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
			type updateItem struct {
				skill      manager.InstalledSkill
				remoteSHA  string
				tmpDir     string
				skills     []manager.DiscoveredSkill
			}
			var toUpdate []updateItem
			var upToDate []string

			for _, s := range installed {
				k := sourceKey{s.Meta.CloneURL, s.Meta.Ref}
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
			type clonedSource struct {
				tmpDir    string
				skills    []manager.DiscoveredSkill
				commitSHA string
			}
			cloned := make(map[sourceKey]*clonedSource)

			for i := range toUpdate {
				item := &toUpdate[i]
				k := sourceKey{item.skill.Meta.CloneURL, item.skill.Meta.Ref}

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
					skills := manager.DiscoverSkills(tmpDir)

					cs = &clonedSource{tmpDir: tmpDir, skills: skills, commitSHA: commitSHA}
					cloned[k] = cs
				}

				item.tmpDir = cs.tmpDir
				item.skills = cs.skills
			}

			// Perform updates
			updated := 0
			for _, item := range toUpdate {
				// Find the matching skill in the cloned repo
				var found *manager.DiscoveredSkill
				for _, s := range item.skills {
					if s.Name == item.skill.Meta.SkillName {
						found = &s
						break
					}
				}
				if found == nil {
					fmt.Printf("  %s: skill %q not found in source (skipped)\n", item.skill.Name, item.skill.Meta.SkillName)
					continue
				}

				// Remove old, copy new
				if err := os.RemoveAll(item.skill.Dir); err != nil {
					fmt.Printf("  %s: remove failed: %v\n", item.skill.Name, err)
					continue
				}

				// copyDir needs the destination parent to exist
				if err := os.MkdirAll(filepath.Dir(item.skill.Dir), 0755); err != nil {
					fmt.Printf("  %s: mkdir failed: %v\n", item.skill.Name, err)
					continue
				}

				if err := manager.CopyDir(found.Path, item.skill.Dir); err != nil {
					fmt.Printf("  %s: copy failed: %v\n", item.skill.Name, err)
					continue
				}

				// Re-write metadata with updated SHA
				meta := item.skill.Meta
				meta.CommitSHA = cloned[sourceKey{item.skill.Meta.CloneURL, item.skill.Meta.Ref}].commitSHA
				if err := manager.WriteMeta(item.skill.Dir, meta); err != nil {
					fmt.Printf("  %s: write meta failed: %v\n", item.skill.Name, err)
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
