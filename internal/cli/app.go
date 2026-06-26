package cli

import (
	"github.com/spf13/cobra"
)

func New(version string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "enbu",
		Short:        "Keyless .env management powered by GitHub",
		Version:      version,
		SilenceUsage: true,
	}

	rootCmd.AddCommand(
		newAuthCommand(),
		newInitCommand(),
		newAddCommand(),
		newPullCommand(),
		newSyncCommand(),
	)

	return rootCmd
}
