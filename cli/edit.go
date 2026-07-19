package cli

import (
	"fmt"

	"github.com/enbu-net/enbu/app"
	"github.com/spf13/cobra"
)

func newEditCommand(a *app.App) *cobra.Command {
	var envName string

	cmd := &cobra.Command{
		Use:   "edit KEY VALUE",
		Short: "Edit an existing secret in the repository",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.EditSecret(cmd.Context(), envName, args[0], args[1]); err != nil {
				return err
			}
			fmt.Printf("✓ Edited %s\n", args[0])
			return nil
		},
	}

	cmd.Flags().StringVarP(&envName, "env", "e", "", "Environment to use (overrides current)")
	return cmd
}
