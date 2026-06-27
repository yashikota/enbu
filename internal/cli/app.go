package cli

import (
	"github.com/spf13/cobra"
)

func New(version string) *cobra.Command {
	svc := DefaultService()

	rootCmd := &cobra.Command{
		Use:          "enbu",
		Short:        "Keyless .env management powered by GitHub",
		Version:      version,
		SilenceUsage: true,
	}

	rootCmd.AddCommand(
		newAuthCommand(),
		newInitCommand(svc),
		newAddCommand(svc),
		newPullCommand(svc),
		newSyncCommand(svc),
	)

	return rootCmd
}
