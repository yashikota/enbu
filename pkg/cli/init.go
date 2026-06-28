package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	agecrypto "filippo.io/age"
	"github.com/spf13/cobra"
	"github.com/yashikota/enbu/pkg/age"
	"github.com/yashikota/enbu/pkg/bundle"
	"github.com/yashikota/enbu/pkg/config"
	"github.com/yashikota/enbu/pkg/keystore"
	"github.com/yashikota/enbu/pkg/oci"
	gh "github.com/yashikota/enbu/pkg/provider/github"
)

func newInitCommand(svc *Service) *cobra.Command {
	var envName string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize enbu for this repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			envName = normalizeEnvironmentName(envName)
			if !config.ValidEnvironmentName(envName) {
				return fmt.Errorf("invalid environment %q", envName)
			}

			accessToken, username, err := svc.TokenProvider.LoadToken()
			if err != nil {
				return err
			}

			owner, repo, err := svc.RepoDetector.LoadRepo()
			if err != nil {
				return fmt.Errorf("detecting repository: %w (run inside a git repository)", err)
			}

			registryRef := svc.registryRef(owner, repo)
			secretsRef := svc.secretsRef(owner, repo, envName)

			projectCfg, err := config.LoadProject()
			configMissing := false
			if err != nil {
				if strings.Contains(err.Error(), "enbu.toml not found") {
					configMissing = true
					projectCfg = config.NewProjectWithEnvironment(envName)
				} else {
					return err
				}
			} else if _, err := projectCfg.Environment(envName); err != nil {
				return err
			}
			knownEnvs := projectCfg.EnvironmentNames()

			repoRoot, err := config.RepoRoot()
			if err != nil {
				return fmt.Errorf("finding repository root: %w", err)
			}

			existingTags, err := svc.Registry.ListTags(ctx, registryRef, accessToken)
			if err != nil && !strings.Contains(err.Error(), "404") && !strings.Contains(err.Error(), "NAME_UNKNOWN") {
				return fmt.Errorf("checking existing setup: %w", err)
			}
			hasRecipients := false
			hasSecrets := false
			for _, tag := range existingTags {
				if isUserRecipientTagForEnv(tag, envName, knownEnvs) {
					hasRecipients = true
				}
				if tag == secretsTag(envName) {
					hasSecrets = true
				}
			}

			joinMode := hasRecipients || hasSecrets
			if joinMode {
				fmt.Println("Existing enbu setup detected for this repository.")
				fmt.Printf("Entering join mode for %s — registering your key only.\n", envName)
			}

			repoKey := repoKeystoreKey(owner, repo)
			var publicKey string

			existingPriv, err := svc.KeyStore.Load(keystoreService, repoKey)
			if err == nil && len(existingPriv) > 0 {
				id, err := agecrypto.ParseX25519Identity(string(existingPriv))
				if err != nil {
					return fmt.Errorf("parsing existing private key: %w", err)
				}
				publicKey = id.Recipient().String()
				fmt.Printf("Using existing age public key: %s\n", publicKey)
			} else if err != nil && !errors.Is(err, keystore.ErrNotFound) {
				return fmt.Errorf("loading private key from keystore: %w", err)
			} else {
				fmt.Println("Generating new age key pair...")
				kp, err := age.GenerateKeyPair()
				if err != nil {
					return fmt.Errorf("generating age key pair: %w", err)
				}
				publicKey = kp.PublicKey
				fmt.Printf("Generated age public key: %s\n", publicKey)

				if err := svc.KeyStore.Store(keystoreService, repoKey, []byte(kp.Identity.String())); err != nil {
					return fmt.Errorf("storing private key: %w", err)
				}
			}

			fingerprint := age.Fingerprint(publicKey)
			tag := oci.CleanTag(fmt.Sprintf("%s-%s", username, fingerprint))
			ref := fmt.Sprintf("%s:%s%s", registryRef, recipientTagPrefix(envName), tag)
			fmt.Println("Pushing public key to registry...")
			pushOpts := &oci.PushOptions{
				SourceRepo: fmt.Sprintf("https://github.com/%s/%s", owner, repo),
			}
			if err := svc.Registry.Push(ctx, ref, "application/vnd.enbu.recipient.age.v1", []byte(publicKey), accessToken, pushOpts); err != nil {
				return fmt.Errorf("pushing public key to GHCR: %w", err)
			}
			fmt.Println("✓ Registered user public key.")

			if joinMode {
				fmt.Println("\nYour key has been registered.")
				if hasSecrets {
					identities, err := loadIdentitiesForRepo(svc.KeyStore, owner, repo)
					if err != nil || len(identities) == 0 {
						fmt.Println("Could not load decryption keys; run 'enbu pull' after an existing member runs 'enbu sync'.")
						return nil
					}
					ok, err := verifyCurrentUserCanDecrypt(ctx, svc.Registry, secretsRef, accessToken, identities)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to verify decryption: %v\n", err)
						fmt.Println("Your key is registered, but we couldn't verify if you can decrypt the secrets.")
					} else if !ok {
						fmt.Println("Your key is registered, but the existing secrets have not been re-encrypted for it yet.")
						fmt.Println("Ask an existing member to run 'enbu sync', then run 'enbu pull'.")
					} else {
						fmt.Println("✓ You can now run 'enbu pull' to access secrets.")
					}
				} else {
					fmt.Println("No secrets exist yet. You can access them after a member runs 'enbu add'.")
				}
				ensureProjectGitignore(repoRoot, projectCfg)
				return nil
			}

			if !configMissing {
				ensureProjectGitignore(repoRoot, projectCfg)
				fmt.Printf("\nInitialized %s environment for this repository.\n", envName)
				return nil
			}

			if configMissing {
				if err := config.SaveProject(projectCfg); err != nil {
					return fmt.Errorf("creating enbu.toml: %w", err)
				}
				fmt.Println("✓ Created enbu.toml")
			}

			ensureProjectGitignore(repoRoot, projectCfg)

			fmt.Println("Committing generated files...")
			if err := gitCommitInitFiles(repoRoot); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: auto-commit failed: %v\n", err)
				fmt.Println("  Please commit enbu.toml and .gitignore manually.")
			} else {
				fmt.Println("✓ Committed enbu.toml and .gitignore")
			}

			fmt.Println("\nInitialization complete!")
			fmt.Println("")
			fmt.Println("Configure the package at:")

			ghClient := svc.GitHub
			if ghClient == nil {
				ghClient = gh.NewClient(accessToken)
			}
			if ghClient.IsOrganization(ctx, owner) {
				fmt.Printf("  https://github.com/orgs/%s/packages/container/%s-enbu/settings\n", owner, repo)
			} else {
				fmt.Printf("  https://github.com/users/%s/packages/container/%s-enbu/settings\n", owner, repo)
			}
			fmt.Println("")
			fmt.Println("  1. Inherited access: set to \"Inherit access from source repository (recommended)\"")
			fmt.Println("")
			fmt.Println("Then:")
			fmt.Println("  2. Push the commit: git push")
			fmt.Println("  3. Run 'enbu add KEY VALUE' to add secrets")
			return nil
		},
	}

	addEnvironmentFlag(cmd, &envName)
	return cmd
}

var gitignoreEntries = []string{
	".env",
	".env.*",
	"!.env.example",
}

func ensureProjectGitignore(repoRoot string, cfg *config.ProjectConfig) {
	if err := ensureGitignore(repoRoot, projectGitignoreEntries(cfg)...); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to update .gitignore: %v\n", err)
	} else {
		fmt.Println("✓ Updated .gitignore")
	}
}

func projectGitignoreEntries(cfg *config.ProjectConfig) []string {
	var entries []string
	for _, name := range cfg.EnvironmentNames() {
		env, err := cfg.Environment(name)
		if err != nil {
			continue
		}
		output := gitignorePatternForOutput(env.Output)
		if output != "" {
			entries = append(entries, output)
		}
	}
	return entries
}

func gitignorePatternForOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" || filepath.IsAbs(output) {
		return ""
	}
	if strings.HasPrefix(output, "#") || strings.HasPrefix(output, "!") {
		return `\` + output
	}
	return output
}

func ensureGitignore(repoRoot string, extraEntries ...string) error {
	path := filepath.Join(repoRoot, ".gitignore")

	existing := ""
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	}

	lines := strings.Split(existing, "\n")
	lineSet := make(map[string]bool)
	for _, l := range lines {
		lineSet[strings.TrimSpace(l)] = true
	}

	entries := append([]string{}, gitignoreEntries...)
	entries = append(entries, extraEntries...)

	var toAdd []string
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if !lineSet[entry] {
			toAdd = append(toAdd, entry)
			lineSet[entry] = true
		}
	}

	if len(toAdd) == 0 {
		return nil
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	if existing != "" && !strings.HasSuffix(existing, "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}

	content := "\n# enbu - exclude .env files\n" + strings.Join(toAdd, "\n") + "\n"
	_, err = f.WriteString(content)
	return err
}

func gitCommitInitFiles(repoRoot string) error {
	addCmd := exec.Command("git", "add", "enbu.toml", ".gitignore")
	addCmd.Dir = repoRoot
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %s: %w", out, err)
	}

	commitCmd := exec.Command("git", "commit", "-m", "chore: add enbu config")
	commitCmd.Dir = repoRoot
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %s: %w", out, err)
	}

	return nil
}

func isUserRecipientTag(tag string) bool {
	return isUserRecipientTagForEnv(tag, defaultEnvironment, []string{defaultEnvironment})
}

func pullAllRecipients(ctx context.Context, reg Registry, ref string, token string, env string, knownEnvs []string) ([]string, error) {
	tags, err := reg.ListTags(ctx, ref, token)
	if err != nil {
		return nil, err
	}

	var publicKeys []string
	for _, tag := range tags {
		if !isUserRecipientTagForEnv(tag, env, knownEnvs) {
			continue
		}
		tagRef := fmt.Sprintf("%s:%s", ref, tag)
		data, err := reg.Pull(ctx, tagRef, token)
		if err != nil {
			return nil, fmt.Errorf("pulling recipient %s: %w", tag, err)
		}
		publicKeys = append(publicKeys, string(data))
	}
	return publicKeys, nil
}

func secretsExists(ctx context.Context, reg Registry, ref, token string) bool {
	_, err := reg.GetDigest(ctx, ref, token)
	return err == nil
}

func pullSecretsWithDigest(ctx context.Context, reg Registry, ref, token string, identities ...agecrypto.Identity) (map[string]string, string, error) {
	digest, err := reg.GetDigest(ctx, ref, token)
	if err != nil {
		return nil, "", err
	}

	ciphertext, err := reg.Pull(ctx, ref, token)
	if err != nil {
		return nil, "", err
	}

	plaintext, err := age.Decrypt(ciphertext, identities...)
	if err != nil {
		return nil, "", err
	}

	secrets, err := bundle.Unmarshal(plaintext)
	if err != nil {
		return nil, "", err
	}

	return secrets, digest, nil
}

func verifyCurrentUserCanDecrypt(ctx context.Context, reg Registry, secretsRef, token string, identities []agecrypto.Identity) (bool, error) {
	ciphertext, err := reg.Pull(ctx, secretsRef, token)
	if err != nil {
		return false, err
	}
	_, err = age.Decrypt(ciphertext, identities...)
	if err != nil {
		return false, nil
	}
	return true, nil
}
