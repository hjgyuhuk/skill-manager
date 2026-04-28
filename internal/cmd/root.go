package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"skillman/internal/manager"

	"github.com/spf13/cobra"
)

func NewRootCmd(m *manager.Manager) *cobra.Command {
	root := &cobra.Command{
		Use:          "skillman",
		Short:        "Manage agent skills",
		Long:         "skillman manages skills across multiple agent directories",
		SilenceUsage: true,
	}

	root.AddCommand(
		newListCmd(m),
		newDisableCmd(m),
		newEnableCmd(m),
		newUninstallCmd(m),
	)

	return root
}

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

func newDisableCmd(m *manager.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name-or-pattern> [...]",
		Short: "Disable skill(s) by name or glob pattern",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			matches, err := m.ResolvePattern(args[0])
			if err != nil {
				return err
			}
			for _, pattern := range args[1:] {
				more, err := m.ResolvePattern(pattern)
				if err != nil {
					return err
				}
				matches = append(matches, more...)
			}
			matches = dedupMatches(matches)

			// Filter to only enabled ones
			var enabled []manager.Match
			for _, match := range matches {
				src := match.Site.SkillsDir + "/" + match.Name
				if _, err := os.Stat(src); err == nil {
					enabled = append(enabled, match)
				}
			}

			if len(enabled) == 0 {
				return fmt.Errorf("no enabled skills match %v", args)
			}

			if needConfirm(args, enabled) {
				if !confirmMatchAction("disable", enabled) {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			return m.Disable(enabled)
		},
	}
}

func newEnableCmd(m *manager.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <name-or-pattern> [...]",
		Short: "Enable skill(s) by name or glob pattern",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			matches, err := m.ResolvePattern(args[0])
			if err != nil {
				return err
			}
			for _, pattern := range args[1:] {
				more, err := m.ResolvePattern(pattern)
				if err != nil {
					return err
				}
				matches = append(matches, more...)
			}
			matches = dedupMatches(matches)

			// Filter to only disabled ones
			var disabled []manager.Match
			for _, match := range matches {
				src := match.Site.DisabledDir + "/" + match.Name
				if _, err := os.Stat(src); err == nil {
					disabled = append(disabled, match)
				}
			}

			if len(disabled) == 0 {
				return fmt.Errorf("no disabled skills match %v", args)
			}

			if needConfirm(args, disabled) {
				if !confirmMatchAction("enable", disabled) {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			return m.Enable(disabled)
		},
	}
}

func newUninstallCmd(m *manager.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall <name-or-pattern> [...]",
		Short: "Uninstall (delete) skill(s) by name or glob pattern",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			matches, err := m.ResolvePattern(args[0])
			if err != nil {
				return err
			}
			for _, pattern := range args[1:] {
				more, err := m.ResolvePattern(pattern)
				if err != nil {
					return err
				}
				matches = append(matches, more...)
			}
			matches = dedupMatches(matches)

			if len(matches) == 0 {
				return fmt.Errorf("no skills match %v", args)
			}

			if !confirmMatchAction("uninstall (DELETE)", matches) {
				fmt.Println("Cancelled.")
				return nil
			}

			return m.Uninstall(matches)
		},
	}
}

func needConfirm(args []string, matches []manager.Match) bool {
	if len(args) > 1 {
		return true
	}
	if len(matches) > 1 {
		return true
	}
	for _, a := range args {
		if isGlobPattern(a) {
			return true
		}
	}
	return false
}

func isGlobPattern(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

func confirmMatchAction(action string, matches []manager.Match) bool {
	fmt.Printf("Will %s %d skill(s):\n", action, len(matches))
	for _, match := range matches {
		fmt.Printf("  - %s (%s)\n", match.Name, match.SiteName)
	}

	fmt.Printf("\nProceed? [y/N] ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

func dedupMatches(items []manager.Match) []manager.Match {
	seen := make(map[string]struct{})
	var result []manager.Match
	for _, item := range items {
		key := item.Name + "\x00" + item.SiteName
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

func Execute() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	m := manager.New(home)

	if err := NewRootCmd(m).Execute(); err != nil {
		os.Exit(1)
	}
}
