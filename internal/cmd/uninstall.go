package cmd

import (
	"fmt"

	"skillman/internal/manager"

	"github.com/spf13/cobra"
)

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
