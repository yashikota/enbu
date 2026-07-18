package app

import (
	"context"
	"errors"
	agecrypto "filippo.io/age"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"time"

	"github.com/yashikota/enbu/utils/age"
	"github.com/yashikota/enbu/utils/bundle"
	"github.com/yashikota/enbu/utils/oci"
)

const maxRetries = 3

func (a *App) ListSecrets(ctx context.Context, env string) (map[string]string, error) {
	resolved, err := a.resolveEnvironment(env)
	if err != nil {
		return nil, err
	}

	accessToken, _, err := a.TokenProvider.LoadToken()
	if err != nil {
		return nil, err
	}

	owner, repo, err := a.RepoDetector.LoadRepo()
	if err != nil {
		return nil, err
	}

	identities, err := LoadIdentitiesForRepo(a.KeyStore, owner, repo)
	if err != nil {
		return nil, err
	}
	if len(identities) == 0 {
		return nil, fmt.Errorf("no decryption keys found (run 'enbu init' first)")
	}

	secretsRef := a.secretsRef(owner, repo, resolved.Name)

	secrets, _, err := PullSecretsWithDigest(ctx, a.Registry, secretsRef, accessToken, identities...)
	if err != nil {
		if IsNotFoundError(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("pulling secrets: %w", err)
	}

	return secrets, nil
}

func (a *App) AddSecret(ctx context.Context, env, key, value string) error {
	resolved, err := a.resolveEnvironment(env)
	if err != nil {
		return err
	}

	accessToken, _, err := a.TokenProvider.LoadToken()
	if err != nil {
		return err
	}

	owner, repo, err := a.RepoDetector.LoadRepo()
	if err != nil {
		return err
	}

	identities, err := LoadIdentitiesForRepo(a.KeyStore, owner, repo)
	if err != nil {
		return err
	}
	if len(identities) == 0 {
		return fmt.Errorf("no decryption keys found (run 'enbu init' first)")
	}

	secretsRef := a.secretsRef(owner, repo, resolved.Name)
	recipientsRef := a.registryRef(owner, repo)

	publicKeys, err := PullAllRecipients(ctx, a.Registry, recipientsRef, accessToken)
	if err != nil {
		return fmt.Errorf("pulling recipients: %w", err)
	}
	if len(publicKeys) == 0 {
		return fmt.Errorf("no recipients found (has anyone run 'enbu init'?)")
	}

	pushOpts := &oci.PushOptions{
		SourceRepo: a.sourceRepoURL(owner, repo),
	}

	for attempt := range maxRetries {
		secrets, baseDigest, err := PullSecretsWithDigest(ctx, a.Registry, secretsRef, accessToken, identities...)
		if err != nil {
			if !IsNotFoundError(err) {
				return fmt.Errorf("pulling secrets: %w", err)
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

		if err := a.Registry.Push(ctx, secretsRef, "application/vnd.enbu.secrets.age.v1", ciphertext, accessToken, pushOpts); err != nil {
			if errors.Is(err, oci.ErrConflict) {
				if attempt < maxRetries-1 {
					a.emitRetry(attempt+1, maxRetries)
					time.Sleep(time.Duration(100+rand.IntN(100)) * time.Millisecond)
					continue
				}
				return fmt.Errorf("secrets changed by another user, failed after %d attempts", maxRetries)
			}
			return fmt.Errorf("pushing encrypted secrets: %w", err)
		}

		_ = a.Registry.Push(ctx, fmt.Sprintf("%s:%s", a.registryRef(owner, repo), snapshotTag(resolved.Name)), "application/vnd.enbu.secrets.age.v1", ciphertext, accessToken, &oci.PushOptions{SourceRepo: a.sourceRepoURL(owner, repo)})
		a.emit(fmt.Sprintf("Added %s (%d secrets total)", key, len(secrets)))
		return nil
	}
	return nil
}

func (a *App) EditSecret(ctx context.Context, env, key, value string) error {
	resolved, err := a.resolveEnvironment(env)
	if err != nil {
		return err
	}

	accessToken, _, err := a.TokenProvider.LoadToken()
	if err != nil {
		return err
	}

	owner, repo, err := a.RepoDetector.LoadRepo()
	if err != nil {
		return err
	}

	identities, err := LoadIdentitiesForRepo(a.KeyStore, owner, repo)
	if err != nil {
		return err
	}
	if len(identities) == 0 {
		return fmt.Errorf("no decryption keys found (run 'enbu init' first)")
	}

	secretsRef := a.secretsRef(owner, repo, resolved.Name)
	recipientsRef := a.registryRef(owner, repo)

	publicKeys, err := PullAllRecipients(ctx, a.Registry, recipientsRef, accessToken)
	if err != nil {
		return fmt.Errorf("pulling recipients: %w", err)
	}
	if len(publicKeys) == 0 {
		return fmt.Errorf("no recipients found (has anyone run 'enbu init'?)")
	}

	pushOpts := &oci.PushOptions{
		SourceRepo: a.sourceRepoURL(owner, repo),
	}

	for attempt := range maxRetries {
		secrets, baseDigest, err := PullSecretsWithDigest(ctx, a.Registry, secretsRef, accessToken, identities...)
		if err != nil {
			return fmt.Errorf("pulling secrets: %w", err)
		}

		if _, ok := secrets[key]; !ok {
			return fmt.Errorf("secret %s does not exist (use 'enbu add %s VALUE' to create it)", key, key)
		}
		secrets[key] = value

		pushOpts.ExpectedDigest = baseDigest

		plaintext := bundle.Marshal(secrets)
		ciphertext, err := age.EncryptForPublicKeys(plaintext, publicKeys)
		if err != nil {
			return fmt.Errorf("encrypting secrets: %w", err)
		}

		if err := a.Registry.Push(ctx, secretsRef, "application/vnd.enbu.secrets.age.v1", ciphertext, accessToken, pushOpts); err != nil {
			if errors.Is(err, oci.ErrConflict) {
				if attempt < maxRetries-1 {
					a.emitRetry(attempt+1, maxRetries)
					continue
				}
				return fmt.Errorf("secrets changed by another user, failed after %d attempts", maxRetries)
			}
			return fmt.Errorf("pushing encrypted secrets: %w", err)
		}

		_ = a.Registry.Push(ctx, fmt.Sprintf("%s:%s", a.registryRef(owner, repo), snapshotTag(resolved.Name)), "application/vnd.enbu.secrets.age.v1", ciphertext, accessToken, &oci.PushOptions{SourceRepo: a.sourceRepoURL(owner, repo)})
		a.emit(fmt.Sprintf("Updated %s (%d secrets total)", key, len(secrets)))
		return nil
	}
	return nil
}

func (a *App) DeleteSecret(ctx context.Context, env, key string) error {
	resolved, err := a.resolveEnvironment(env)
	if err != nil {
		return err
	}

	accessToken, _, err := a.TokenProvider.LoadToken()
	if err != nil {
		return err
	}

	owner, repo, err := a.RepoDetector.LoadRepo()
	if err != nil {
		return err
	}

	identities, err := LoadIdentitiesForRepo(a.KeyStore, owner, repo)
	if err != nil {
		return err
	}
	if len(identities) == 0 {
		return fmt.Errorf("no decryption keys found (run 'enbu init' first)")
	}

	secretsRef := a.secretsRef(owner, repo, resolved.Name)
	recipientsRef := a.registryRef(owner, repo)

	publicKeys, err := PullAllRecipients(ctx, a.Registry, recipientsRef, accessToken)
	if err != nil {
		return fmt.Errorf("pulling recipients: %w", err)
	}
	if len(publicKeys) == 0 {
		return fmt.Errorf("no recipients found (has anyone run 'enbu init'?)")
	}

	pushOpts := &oci.PushOptions{
		SourceRepo: a.sourceRepoURL(owner, repo),
	}

	for attempt := range maxRetries {
		secrets, baseDigest, err := PullSecretsWithDigest(ctx, a.Registry, secretsRef, accessToken, identities...)
		if err != nil {
			if IsNotFoundError(err) {
				return nil
			}
			return fmt.Errorf("pulling secrets: %w", err)
		}

		if _, ok := secrets[key]; !ok {
			return nil
		}
		delete(secrets, key)

		pushOpts.ExpectedDigest = baseDigest

		plaintext := bundle.Marshal(secrets)
		ciphertext, err := age.EncryptForPublicKeys(plaintext, publicKeys)
		if err != nil {
			return fmt.Errorf("encrypting secrets: %w", err)
		}

		if err := a.Registry.Push(ctx, secretsRef, "application/vnd.enbu.secrets.age.v1", ciphertext, accessToken, pushOpts); err != nil {
			if errors.Is(err, oci.ErrConflict) {
				if attempt < maxRetries-1 {
					a.emitRetry(attempt+1, maxRetries)
					continue
				}
				return fmt.Errorf("secrets changed by another user, failed after %d attempts", maxRetries)
			}
			return fmt.Errorf("pushing encrypted secrets: %w", err)
		}

		_ = a.Registry.Push(ctx, fmt.Sprintf("%s:%s", a.registryRef(owner, repo), snapshotTag(resolved.Name)), "application/vnd.enbu.secrets.age.v1", ciphertext, accessToken, &oci.PushOptions{SourceRepo: a.sourceRepoURL(owner, repo)})
		a.emit(fmt.Sprintf("Deleted %s (%d secrets remaining)", key, len(secrets)))
		return nil
	}
	return nil
}

func (a *App) PullSecrets(ctx context.Context, env string) ([]byte, string, int, error) {
	resolved, err := a.resolveEnvironment(env)
	if err != nil {
		return nil, "", 0, err
	}

	accessToken, _, err := a.TokenProvider.LoadToken()
	if err != nil {
		return nil, "", 0, err
	}

	owner, repo, err := a.RepoDetector.LoadRepo()
	if err != nil {
		return nil, "", 0, err
	}

	ref := a.secretsRef(owner, repo, resolved.Name)

	ciphertext, err := a.Registry.Pull(ctx, ref, accessToken)
	if err != nil {
		return nil, "", 0, fmt.Errorf("pulling secrets: %w", err)
	}

	identities, err := LoadIdentitiesForRepo(a.KeyStore, owner, repo)
	if err != nil {
		return nil, "", 0, err
	}
	if len(identities) == 0 {
		return nil, "", 0, fmt.Errorf("no decryption keys found (run 'enbu init' first)")
	}

	plaintext, err := age.Decrypt(ciphertext, identities...)
	if err != nil {
		return nil, "", 0, fmt.Errorf("decrypting secrets: %w", err)
	}

	secrets, err := bundle.Unmarshal(plaintext)
	if err != nil {
		return nil, "", 0, fmt.Errorf("parsing secrets: %w", err)
	}

	dotenv := bundle.ToDotEnv(secrets)
	return dotenv, resolved.Output, len(secrets), nil
}

func (a *App) PullSecretsToFile(ctx context.Context, env string) error {
	dotenv, output, count, err := a.PullSecrets(ctx, env)
	if err != nil {
		return err
	}

	outputPath := output
	if a.RepositoryDir != "" && !filepath.IsAbs(output) {
		outputPath = filepath.Join(a.RepositoryDir, output)
	}
	if err := os.WriteFile(outputPath, dotenv, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", output, err)
	}

	a.emit(fmt.Sprintf("Written %s (%d secrets)", output, count))
	return nil
}

var errConflict = errors.New("secrets changed by another user")

func (a *App) SyncSecrets(ctx context.Context, env string) error {
	resolved, err := a.resolveEnvironment(env)
	if err != nil {
		return err
	}

	accessToken, _, err := a.TokenProvider.LoadToken()
	if err != nil {
		return err
	}

	owner, repo, err := a.RepoDetector.LoadRepo()
	if err != nil {
		return err
	}

	identities, err := LoadIdentitiesForRepo(a.KeyStore, owner, repo)
	if err != nil {
		return err
	}
	if len(identities) == 0 {
		return fmt.Errorf("no decryption keys found (run 'enbu init' first)")
	}

	secretsRef := a.secretsRef(owner, repo, resolved.Name)
	recipientsRef := a.registryRef(owner, repo)
	pushOpts := &oci.PushOptions{
		SourceRepo: a.sourceRepoURL(owner, repo),
	}

	const syncMaxRetries = 5
	backoff := 1 * time.Second

	for attempt := range syncMaxRetries {
		err := a.doSync(ctx, secretsRef, recipientsRef, accessToken, identities, pushOpts)
		if err == nil {
			return nil
		}
		if !errors.Is(err, errConflict) {
			return err
		}
		if attempt == syncMaxRetries-1 {
			return fmt.Errorf("sync failed after %d attempts: %w", syncMaxRetries, err)
		}

		a.emitRetry(attempt+1, syncMaxRetries)

		jitter := time.Duration(rand.Int64N(int64(backoff / 2)))
		wait := backoff + jitter

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		backoff *= 2
	}
	return nil
}

func (a *App) doSync(ctx context.Context, secretsRef, recipientsRef, token string, identities []agecrypto.Identity, pushOpts *oci.PushOptions) error {
	secrets, baseDigest, err := PullSecretsWithDigest(ctx, a.Registry, secretsRef, token, identities...)
	if err != nil {
		if IsNotFoundError(err) {
			a.emit("No secrets found, nothing to sync.")
			return nil
		}
		return fmt.Errorf("pulling secrets: %w", err)
	}

	publicKeys, err := PullAllRecipients(ctx, a.Registry, recipientsRef, token)
	if err != nil {
		return fmt.Errorf("pulling recipients: %w", err)
	}
	if len(publicKeys) == 0 {
		return fmt.Errorf("no recipients found")
	}

	if baseDigest != "" {
		currentDigest, err := a.Registry.GetDigest(ctx, secretsRef, token)
		if err == nil && currentDigest != baseDigest {
			return fmt.Errorf("%w", errConflict)
		}
	}

	plaintext := bundle.Marshal(secrets)
	ciphertext, err := age.EncryptForPublicKeys(plaintext, publicKeys)
	if err != nil {
		return fmt.Errorf("encrypting secrets: %w", err)
	}

	if err := a.Registry.Push(ctx, secretsRef, "application/vnd.enbu.secrets.age.v1", ciphertext, token, pushOpts); err != nil {
		return fmt.Errorf("pushing encrypted secrets: %w", err)
	}

	a.emit(fmt.Sprintf("Synchronized secrets for %d recipients (%d secrets)", len(publicKeys), len(secrets)))
	return nil
}
