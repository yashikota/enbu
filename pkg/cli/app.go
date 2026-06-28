package cli

import (
	"github.com/spf13/cobra"
)

func New(version string) *cobra.Command {
	return NewWithService(version, DefaultService())
}

func NewWithService(version string, svc *Service) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "enbu",
		Short:        "Keyless .env management powered by GitHub",
		Version:      version,
		SilenceUsage: true,
	}

	rootCmd.AddCommand(
		newAuthCommand(),
		newInitCommand(svc),
		newSwitchCommand(svc),
		newAddCommand(svc),
		newEditCommand(svc),
		newDeleteCommand(svc),
		newPullCommand(svc),
		newSyncCommand(svc),
	)

	return rootCmd
}
