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

const maxRetries = 3

func newAddCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add KEY VALUE",
		Short: "Add a secret to the repository",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			key := args[0]
			value := args[1]

			token, err := auth.LoadToken()
			if err != nil {
				return err
			}

			cfg, err := config.LoadRepo()
			if err != nil {
				return err
			}

			identities, err := loadIdentitiesForRepo(cfg)
			if err != nil || len(identities) == 0 {
				return fmt.Errorf("no decryption keys found (run 'enbu init' first)")
			}

			secretsRef := fmt.Sprintf("ghcr.io/%s/%s-enbu:secrets-default", strings.ToLower(cfg.Owner), strings.ToLower(cfg.Repo))
			recipientsRef := fmt.Sprintf("ghcr.io/%s/%s-enbu", strings.ToLower(cfg.Owner), strings.ToLower(cfg.Repo))

			publicKeys, err := pullAllRecipients(ctx, recipientsRef, token.AccessToken)
			if err != nil {
				return fmt.Errorf("pulling recipients: %w", err)
			}
			if len(publicKeys) == 0 {
				return fmt.Errorf("no recipients found (has anyone run 'enbu init'?)")
			}

			pushOpts := &oci.PushOptions{
				SourceRepo: fmt.Sprintf("https://github.com/%s/%s", cfg.Owner, cfg.Repo),
			}

			for attempt := range maxRetries {
				secrets, baseDigest, err := pullSecretsWithDigest(ctx, secretsRef, token.AccessToken, identities...)
				if err != nil {
					if secretsExists(ctx, secretsRef, token.AccessToken) {
						return fmt.Errorf("cannot decrypt existing secrets: %w", err)
					}
					secrets = make(map[string]string)
					baseDigest = ""
				}

				secrets[key] = value

				if baseDigest != "" {
					currentDigest, err := oci.GetDigest(ctx, secretsRef, token.AccessToken)
					if err == nil && currentDigest != baseDigest {
						if attempt < maxRetries-1 {
							fmt.Fprintf(os.Stderr, "Conflict detected, retrying (%d/%d)...\n", attempt+1, maxRetries)
							continue
						}
						return fmt.Errorf("secrets changed by another user, failed after %d attempts", maxRetries)
					}
				}

				plaintext := bundle.Marshal(secrets)
				ciphertext, err := age.EncryptForPublicKeys(plaintext, publicKeys)
				if err != nil {
					return fmt.Errorf("encrypting secrets: %w", err)
				}

				if err := oci.Push(ctx, secretsRef, "application/vnd.enbu.secrets.age.v1", ciphertext, token.AccessToken, pushOpts); err != nil {
					return fmt.Errorf("pushing encrypted secrets: %w", err)
				}

				fmt.Printf("✓ Added %s (%d secrets total)\n", key, len(secrets))
				return nil
			}
			return nil
		},
	}

	return cmd
}
