package cli

import (
	"fmt"
	"os"

	"github.com/enbu-net/enbu/app"
	"github.com/spf13/cobra"
)

func newPullCommand(a *app.App) *cobra.Command {
	var toStdout bool
	var envName string

	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull and decrypt secrets into .env",
		RunE: func(cmd *cobra.Command, args []string) error {
			dotenv, output, count, err := a.PullSecrets(cmd.Context(), envName)
			if err != nil {
				return err
			}

			if toStdout {
				_, _ = os.Stdout.Write(dotenv)
				return nil
			}

			if err := os.WriteFile(output, dotenv, 0o600); err != nil {
				return fmt.Errorf("writing %s: %w", output, err)
			}

			fmt.Fprintf(os.Stderr, "✓ Written %s (%d secrets)\n", output, count)
			return nil
		},
	}

	cmd.Flags().BoolVar(&toStdout, "stdout", false, "Output to stdout instead of .env file")
	cmd.Flags().StringVarP(&envName, "env", "e", "", "Environment to use (overrides current)")
	return cmd
}
