package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yashikota/enbu/app"
)

func newDeleteCommand(a *app.App) *cobra.Command {
	var envName string

	cmd := &cobra.Command{
		Use:   "delete KEY",
		Short: "Delete a secret from the repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.DeleteSecret(cmd.Context(), envName, args[0]); err != nil {
				return err
			}
			fmt.Printf("✓ Deleted %s\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVarP(&envName, "env", "e", "", "Environment to use (overrides current)")
	return cmd
}
