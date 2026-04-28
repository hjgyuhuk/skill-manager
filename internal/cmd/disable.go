package cmd

import (
	"fmt"
	"os"

	"skillman/internal/manager"

	"github.com/spf13/cobra"
)

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
