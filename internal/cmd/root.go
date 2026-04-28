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
		newInstallCmd(m),
		newUpdateCmd(m),
		newDisableCmd(m),
		newEnableCmd(m),
		newUninstallCmd(m),
	)

	return root
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
