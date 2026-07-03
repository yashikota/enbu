package cli

import (
	"github.com/spf13/cobra"
	"github.com/yashikota/enbu/app"
)

func New(version string) *cobra.Command {
	return NewWithApp(version, app.New())
}

func NewWithApp(version string, a *app.App) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "enbu",
		Short:        "Keyless .env management powered by GitHub",
		Version:      version,
		SilenceUsage: true,
	}

	rootCmd.AddCommand(
		newAuthCommand(),
		newInitCommand(a),
		newSwitchCommand(a),
		newAddCommand(a),
		newEditCommand(a),
		newDeleteCommand(a),
		newPullCommand(a),
		newSyncCommand(a),
		newTuiCommand(a),
		newGUICommand(a),
		newHistoryCommand(a),
	)

	return rootCmd
}
