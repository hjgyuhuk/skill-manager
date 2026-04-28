package cmd

import (
	"fmt"
	"strings"

	"skillman/internal/manager"

	"github.com/spf13/cobra"
)

func newListCmd(m *manager.Manager) *cobra.Command {
	var enabledOnly, disabledOnly bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all skills (enabled and disabled)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := m.List()
			if err != nil {
				return err
			}

			// Group by name to detect duplicates
			type skillEntry struct {
				name    string
				sites   []string
				enabled bool
			}

			enabled := make(map[string]*skillEntry)
			disabled := make(map[string]*skillEntry)

			for _, s := range result.Skills {
				var group map[string]*skillEntry
				if s.Enabled {
					group = enabled
				} else {
					group = disabled
				}

				if existing, ok := group[s.Name]; ok {
					existing.sites = append(existing.sites, s.SiteName)
				} else {
					group[s.Name] = &skillEntry{
						name:    s.Name,
						sites:   []string{s.SiteName},
						enabled: s.Enabled,
					}
				}
			}

			showEnabled := !disabledOnly
			showDisabled := !enabledOnly
			total := 0

			if showEnabled && len(enabled) > 0 {
				fmt.Println("Enabled:")
				for _, name := range sortedKeys(enabled) {
					e := enabled[name]
					total++
					if len(e.sites) > 1 {
						fmt.Printf("  ✓ %s  (%s)\n", e.name, strings.Join(e.sites, ", "))
					} else {
						fmt.Printf("  ✓ %s\n", e.name)
					}
				}
			}

			if showDisabled && len(disabled) > 0 {
				if showEnabled && len(enabled) > 0 {
					fmt.Println()
				}
				fmt.Println("Disabled:")
				for _, name := range sortedKeys(disabled) {
					d := disabled[name]
					total++
					if len(d.sites) > 1 {
						fmt.Printf("  ✗ %s  (%s)\n", d.name, strings.Join(d.sites, ", "))
					} else {
						fmt.Printf("  ✗ %s\n", d.name)
					}
				}
			}

			if total == 0 {
				fmt.Println("No skills found.")
				return nil
			}

			fmt.Printf("\nTotal: %d\n", total)
			return nil
		},
	}

	cmd.Flags().BoolVar(&enabledOnly, "enabled", false, "Show only enabled skills")
	cmd.Flags().BoolVar(&disabledOnly, "disabled", false, "Show only disabled skills")

	return cmd
}
