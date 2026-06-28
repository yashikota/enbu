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
	var envName string

	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull and decrypt secrets into .env",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			env, err := resolveCommandEnvironment(envName)
			if err != nil {
				return err
			}

			accessToken, _, err := svc.TokenProvider.LoadToken()
			if err != nil {
				return err
			}

			owner, repo, err := svc.RepoDetector.LoadRepo()
			if err != nil {
				return err
			}

			ref := svc.secretsRef(owner, repo, env.Name)
			fmt.Fprintf(os.Stderr, "Pulling %s secrets...\n", env.Name)

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

			if err := os.WriteFile(env.Output, dotenv, 0o600); err != nil {
				return fmt.Errorf("writing %s: %w", env.Output, err)
			}

			fmt.Fprintf(os.Stderr, "✓ Written %s (%d secrets)\n", env.Output, len(secrets))
			return nil
		},
	}

	cmd.Flags().BoolVar(&toStdout, "stdout", false, "Output to stdout instead of .env file")
	addEnvironmentFlag(cmd, &envName)
	return cmd
}
