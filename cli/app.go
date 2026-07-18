package cli

import (
	"github.com/spf13/cobra"
	"github.com/yashikota/enbu/app"
	gitprovider "github.com/yashikota/enbu/provider/git"
	"github.com/yashikota/enbu/tui"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run(a)
		},
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
		newHistoryCommand(a),
	)

	return rootCmd
}

func gitClient(a *app.App) gitprovider.Client {
	if a.Git != nil {
		return a.Git
	}
	return gitprovider.NewCLIClient()
}
