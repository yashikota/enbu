package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yashikota/enbu/pkg/age"
	"github.com/yashikota/enbu/pkg/bundle"
)

func newPullCommand(svc *Service) *cobra.Command {
	var toStdout bool

	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull and decrypt secrets into .env",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			accessToken, _, err := svc.TokenProvider.LoadToken()
			if err != nil {
				return err
			}

			owner, repo, err := svc.RepoDetector.LoadRepo()
			if err != nil {
				return err
			}

			ref := svc.secretsRef(owner, repo)
			fmt.Fprintf(os.Stderr, "Pulling secrets...\n")

			ciphertext, err := svc.Registry.Pull(ctx, ref, accessToken)
			if err != nil {
				return fmt.Errorf("pulling secrets: %w", err)
			}

			identities, err := loadIdentitiesForRepo(svc.KeyStore, owner, repo)
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
