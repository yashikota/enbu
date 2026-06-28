package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yashikota/enbu/app"
)

func newSyncCommand(a *app.App) *cobra.Command {
	var envName string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Re-encrypt secrets for all current recipients",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.SyncSecrets(cmd.Context(), envName); err != nil {
				return err
			}
			fmt.Println("✓ Sync complete")
			return nil
		},
	}

	cmd.Flags().StringVarP(&envName, "env", "e", "", "Environment to use (overrides current)")
	return cmd
}
