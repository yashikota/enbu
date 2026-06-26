package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yashikota/enbu/internal/age"
	"github.com/yashikota/enbu/internal/auth"
	"github.com/yashikota/enbu/internal/bundle"
	"github.com/yashikota/enbu/internal/config"
	"github.com/yashikota/enbu/internal/oci"
)

func newSyncCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Re-encrypt secrets for all current recipients",
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

			identities, err := loadIdentities(token.AccessToken)
			if err != nil || len(identities) == 0 {
				return fmt.Errorf("no decryption keys found (run 'enbu init' first)")
			}

			secretsRef := fmt.Sprintf("ghcr.io/%s/%s-enbu:secrets-default", strings.ToLower(cfg.Owner), strings.ToLower(cfg.Repo))
			recipientsRef := fmt.Sprintf("ghcr.io/%s/%s-enbu", strings.ToLower(cfg.Owner), strings.ToLower(cfg.Repo))

			secrets, baseDigest, err := pullSecretsWithDigest(ctx, secretsRef, token.AccessToken, identities...)
			if err != nil {
				if !secretsExists(ctx, secretsRef, token.AccessToken) {
					fmt.Println("No secrets found, nothing to sync.")
					return nil
				}
				return fmt.Errorf("pulling secrets: %w", err)
			}

			publicKeys, err := pullAllRecipients(ctx, recipientsRef, token.AccessToken)
			if err != nil {
				return fmt.Errorf("pulling recipients: %w", err)
			}
			if len(publicKeys) == 0 {
				return fmt.Errorf("no recipients found")
			}

			if baseDigest != "" {
				currentDigest, err := oci.GetDigest(ctx, secretsRef, token.AccessToken)
				if err == nil && currentDigest != baseDigest {
					return fmt.Errorf("secrets changed by another user, try again")
				}
			}

			plaintext := bundle.Marshal(secrets)
			ciphertext, err := age.EncryptForPublicKeys(plaintext, publicKeys)
			if err != nil {
				return fmt.Errorf("encrypting secrets: %w", err)
			}

			pushOpts := &oci.PushOptions{
				SourceRepo: fmt.Sprintf("https://github.com/%s/%s", cfg.Owner, cfg.Repo),
			}
			if err := oci.Push(ctx, secretsRef, "application/vnd.enbu.secrets.age.v1", ciphertext, token.AccessToken, pushOpts); err != nil {
				return fmt.Errorf("pushing encrypted secrets: %w", err)
			}

			fmt.Printf("✓ Synchronized secrets for %d recipients (%d secrets)\n", len(publicKeys), len(secrets))
			return nil
		},
	}

	return cmd
}
