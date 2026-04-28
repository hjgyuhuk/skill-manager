package cmd

import (
	"fmt"
	"os"

	"skillman/internal/manager"

	"github.com/spf13/cobra"
)

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
