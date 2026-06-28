package cli

import (
	"github.com/spf13/cobra"
	"github.com/yashikota/enbu/app"
	"github.com/yashikota/enbu/tui"
)

func newTuiCommand(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch interactive terminal UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run(a)
		},
	}
}
