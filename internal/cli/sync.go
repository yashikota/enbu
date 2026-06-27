package cli

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	agecrypto "filippo.io/age"
	"github.com/spf13/cobra"
	"github.com/yashikota/enbu/internal/age"
	"github.com/yashikota/enbu/internal/auth"
	"github.com/yashikota/enbu/internal/bundle"
	"github.com/yashikota/enbu/internal/config"
	"github.com/yashikota/enbu/internal/oci"
)

var errConflict = errors.New("secrets changed by another user")

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

			identities, err := loadIdentitiesForRepo(cfg)
			if err != nil || len(identities) == 0 {
				return fmt.Errorf("no decryption keys found (run 'enbu init' first)")
			}

			secretsRef := fmt.Sprintf("ghcr.io/%s/%s-enbu:secrets-default", strings.ToLower(cfg.Owner), strings.ToLower(cfg.Repo))
			recipientsRef := fmt.Sprintf("ghcr.io/%s/%s-enbu", strings.ToLower(cfg.Owner), strings.ToLower(cfg.Repo))
			pushOpts := &oci.PushOptions{
				SourceRepo: fmt.Sprintf("https://github.com/%s/%s", cfg.Owner, cfg.Repo),
			}

			const maxRetries = 5
			backoff := 1 * time.Second

			for attempt := range maxRetries {
				err := doSync(ctx, secretsRef, recipientsRef, token.AccessToken, identities, pushOpts)
				if err == nil {
					return nil
				}
				if !errors.Is(err, errConflict) {
					return err
				}
				if attempt == maxRetries-1 {
					return fmt.Errorf("sync failed after %d attempts: %w", maxRetries, err)
				}

				jitter := time.Duration(rand.Int64N(int64(backoff / 2)))
				wait := backoff + jitter
				fmt.Printf("Conflict detected, retrying in %s...\n", wait.Round(time.Millisecond))

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(wait):
				}
				backoff *= 2
			}
			return nil
		},
	}

	return cmd
}

func doSync(ctx context.Context, secretsRef, recipientsRef, token string, identities []agecrypto.Identity, pushOpts *oci.PushOptions) error {
	secrets, baseDigest, err := pullSecretsWithDigest(ctx, secretsRef, token, identities...)
	if err != nil {
		if !secretsExists(ctx, secretsRef, token) {
			fmt.Println("No secrets found, nothing to sync.")
			return nil
		}
		return fmt.Errorf("pulling secrets: %w", err)
	}

	publicKeys, err := pullAllRecipients(ctx, recipientsRef, token)
	if err != nil {
		return fmt.Errorf("pulling recipients: %w", err)
	}
	if len(publicKeys) == 0 {
		return fmt.Errorf("no recipients found")
	}

	if baseDigest != "" {
		currentDigest, err := oci.GetDigest(ctx, secretsRef, token)
		if err == nil && currentDigest != baseDigest {
			return fmt.Errorf("%w", errConflict)
		}
	}

	plaintext := bundle.Marshal(secrets)
	ciphertext, err := age.EncryptForPublicKeys(plaintext, publicKeys)
	if err != nil {
		return fmt.Errorf("encrypting secrets: %w", err)
	}

	if err := oci.Push(ctx, secretsRef, "application/vnd.enbu.secrets.age.v1", ciphertext, token, pushOpts); err != nil {
		return fmt.Errorf("pushing encrypted secrets: %w", err)
	}

	fmt.Printf("✓ Synchronized secrets for %d recipients (%d secrets)\n", len(publicKeys), len(secrets))
	return nil
}
