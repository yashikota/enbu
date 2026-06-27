package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yashikota/enbu/internal/age"
	"github.com/yashikota/enbu/internal/auth"
	"github.com/yashikota/enbu/internal/bundle"
	"github.com/yashikota/enbu/internal/config"
	"github.com/yashikota/enbu/internal/oci"
)

func newPullCommand() *cobra.Command {
	var toStdout bool

	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull and decrypt secrets into .env",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			token, err := auth.LoadToken()
			if err != nil {
				return err
			}

			cfg, err := config.LoadRepo()
			if err != nil {
				return err
			}

			ref := fmt.Sprintf("ghcr.io/%s/%s-enbu:secrets-default", strings.ToLower(cfg.Owner), strings.ToLower(cfg.Repo))
			fmt.Fprintf(os.Stderr, "Pulling secrets...\n")

			ciphertext, err := oci.Pull(ctx, ref, token.AccessToken)
			if err != nil {
				return fmt.Errorf("pulling secrets: %w", err)
			}

			identities, err := loadIdentitiesForRepo(cfg)
			if err != nil || len(identities) == 0 {
				return fmt.Errorf("no decryption keys found (run 'enbu init' first)")
			}

			plaintext, err := age.Decrypt(ciphertext, identities...)
			if err != nil {
				return fmt.Errorf("decrypting secrets: %w", err)
			}

			secrets, err := bundle.Unmarshal(plaintext)
			if err != nil {
				return fmt.Errorf("parsing secrets: %w", err)
			}

			dotenv := bundle.ToDotEnv(secrets)

			if toStdout {
				_, _ = os.Stdout.Write(dotenv)
				return nil
			}

			if err := os.WriteFile(".env", dotenv, 0o600); err != nil {
				return fmt.Errorf("writing .env: %w", err)
			}

			fmt.Fprintf(os.Stderr, "✓ Written .env (%d secrets)\n", len(secrets))
			return nil
		},
	}

	cmd.Flags().BoolVar(&toStdout, "stdout", false, "Output to stdout instead of .env file")
	return cmd
}
