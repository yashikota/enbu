package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yashikota/enbu/pkg/age"
	"github.com/yashikota/enbu/pkg/bundle"
	"github.com/yashikota/enbu/pkg/oci"
)

const maxRetries = 3

func newAddCommand(svc *Service) *cobra.Command {
	var envName string

	cmd := &cobra.Command{
		Use:   "add KEY VALUE",
		Short: "Add a secret to the repository",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key := args[0]
			value := args[1]
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

			identities, err := loadIdentitiesForRepo(svc.KeyStore, owner, repo)
			if err != nil || len(identities) == 0 {
				return fmt.Errorf("no decryption keys found (run 'enbu init' first)")
			}

			secretsRef := svc.secretsRef(owner, repo, env.Name)
			recipientsRef := svc.registryRef(owner, repo)

			publicKeys, err := pullAllRecipients(ctx, svc.Registry, recipientsRef, accessToken, env.Name, env.KnownEnvs)
			if err != nil {
				return fmt.Errorf("pulling recipients: %w", err)
			}
			if len(publicKeys) == 0 {
				return fmt.Errorf("no recipients found (has anyone run 'enbu init'?)")
			}

			pushOpts := &oci.PushOptions{
				SourceRepo: fmt.Sprintf("https://github.com/%s/%s", owner, repo),
			}

			for attempt := range maxRetries {
				secrets, baseDigest, err := pullSecretsWithDigest(ctx, svc.Registry, secretsRef, accessToken, identities...)
				if err != nil {
					if secretsExists(ctx, svc.Registry, secretsRef, accessToken) {
						return fmt.Errorf("cannot decrypt existing secrets: %w", err)
					}
					secrets = make(map[string]string)
					baseDigest = ""
				}

				if _, ok := secrets[key]; ok {
					return fmt.Errorf("secret %s already exists (use 'enbu edit %s VALUE' to update it)", key, key)
				}
				secrets[key] = value

				pushOpts.ExpectedDigest = baseDigest

				plaintext := bundle.Marshal(secrets)
				ciphertext, err := age.EncryptForPublicKeys(plaintext, publicKeys)
				if err != nil {
					return fmt.Errorf("encrypting secrets: %w", err)
				}

				if err := svc.Registry.Push(ctx, secretsRef, "application/vnd.enbu.secrets.age.v1", ciphertext, accessToken, pushOpts); err != nil {
					if errors.Is(err, oci.ErrConflict) {
						if attempt < maxRetries-1 {
							fmt.Fprintf(os.Stderr, "Conflict detected, retrying (%d/%d)...\n", attempt+1, maxRetries)
							continue
						}
						return fmt.Errorf("secrets changed by another user, failed after %d attempts", maxRetries)
					}
					return fmt.Errorf("pushing encrypted secrets: %w", err)
				}

				fmt.Printf("✓ Added %s (%d secrets total)\n", key, len(secrets))
				return nil
			}
			return nil
		},
	}

	addEnvironmentFlag(cmd, &envName)
	return cmd
}
