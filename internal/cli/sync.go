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
	"github.com/yashikota/enbu/internal/bundle"
	"github.com/yashikota/enbu/internal/oci"
)

var errConflict = errors.New("secrets changed by another user")

func newSyncCommand(svc *Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Re-encrypt secrets for all current recipients",
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

			identities, err := loadIdentitiesForRepo(svc.KeyStore, owner, repo)
			if err != nil || len(identities) == 0 {
				return fmt.Errorf("no decryption keys found (run 'enbu init' first)")
			}

			secretsRef := svc.secretsRef(owner, repo)
			recipientsRef := svc.registryRef(owner, repo)
			pushOpts := &oci.PushOptions{
				SourceRepo: fmt.Sprintf("https://github.com/%s/%s", owner, repo),
			}

			const syncMaxRetries = 5
			backoff := 1 * time.Second

			for attempt := range syncMaxRetries {
				err := doSync(ctx, svc.Registry, secretsRef, recipientsRef, accessToken, identities, pushOpts)
				if err == nil {
					return nil
				}
				if !errors.Is(err, errConflict) {
					return err
				}
				if attempt == syncMaxRetries-1 {
					return fmt.Errorf("sync failed after %d attempts: %w", syncMaxRetries, err)
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

func doSync(ctx context.Context, reg Registry, secretsRef, recipientsRef, token string, identities []agecrypto.Identity, pushOpts *oci.PushOptions) error {
	secrets, baseDigest, err := pullSecretsWithDigest(ctx, reg, secretsRef, token, identities...)
	if err != nil {
		if isNotFoundError(err) {
			fmt.Println("No secrets found, nothing to sync.")
			return nil
		}
		return fmt.Errorf("pulling secrets: %w", err)
	}

	publicKeys, err := pullAllRecipients(ctx, reg, recipientsRef, token)
	if err != nil {
		return fmt.Errorf("pulling recipients: %w", err)
	}
	if len(publicKeys) == 0 {
		return fmt.Errorf("no recipients found")
	}

	if baseDigest != "" {
		currentDigest, err := reg.GetDigest(ctx, secretsRef, token)
		if err == nil && currentDigest != baseDigest {
			return fmt.Errorf("%w", errConflict)
		}
	}

	plaintext := bundle.Marshal(secrets)
	ciphertext, err := age.EncryptForPublicKeys(plaintext, publicKeys)
	if err != nil {
		return fmt.Errorf("encrypting secrets: %w", err)
	}

	if err := reg.Push(ctx, secretsRef, "application/vnd.enbu.secrets.age.v1", ciphertext, token, pushOpts); err != nil {
		return fmt.Errorf("pushing encrypted secrets: %w", err)
	}

	fmt.Printf("✓ Synchronized secrets for %d recipients (%d secrets)\n", len(publicKeys), len(secrets))
	return nil
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "404") ||
		strings.Contains(errStr, "NAME_UNKNOWN") ||
		strings.Contains(errStr, "not found")
}
