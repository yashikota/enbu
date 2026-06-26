package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	agecrypto "filippo.io/age"
	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/yashikota/enbu/internal/age"
	"github.com/yashikota/enbu/internal/auth"
	"github.com/yashikota/enbu/internal/bundle"
	"github.com/yashikota/enbu/internal/config"
	gh "github.com/yashikota/enbu/internal/github"
	"github.com/yashikota/enbu/internal/oci"
	"github.com/yashikota/enbu/internal/templates"
	"github.com/yashikota/enbu/internal/tokenlock"
)

func newInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize enbu for this repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			token, err := auth.LoadToken()
			if err != nil {
				return err
			}

			cfg, err := config.LoadRepo()
			if err != nil {
				return fmt.Errorf("detecting repository: %w (run inside a git repository)", err)
			}

			registryRef := fmt.Sprintf("ghcr.io/%s/%s-enbu", strings.ToLower(cfg.Owner), strings.ToLower(cfg.Repo))

			repoRoot, err := config.RepoRoot()
			if err != nil {
				return fmt.Errorf("finding repository root: %w", err)
			}

			existingTags, err := oci.ListTags(ctx, registryRef, token.AccessToken)
			if err != nil && !strings.Contains(err.Error(), "404") && !strings.Contains(err.Error(), "NAME_UNKNOWN") {
				return fmt.Errorf("checking existing setup: %w", err)
			}
			hasRecipients := false
			hasSecrets := false
			for _, tag := range existingTags {
				if strings.HasPrefix(tag, "recipient-") {
					hasRecipients = true
				}
				if strings.HasPrefix(tag, "secrets-") {
					hasSecrets = true
				}
			}

			joinMode := hasRecipients || hasSecrets
			if joinMode {
				fmt.Println("Existing enbu setup detected for this repository.")
				fmt.Println("Entering join mode — registering your key only.")
			}

			dataDir := config.DataDir()
			var publicKey string

			if _, err := os.Stat(filepath.Join(dataDir, "age_key.enc")); err == nil {
				pubBytes, err := os.ReadFile(filepath.Join(dataDir, "age_key.pub"))
				if err != nil {
					return fmt.Errorf("reading existing public key: %w", err)
				}
				publicKey = string(pubBytes)
				fmt.Printf("Using existing age public key: %s\n", publicKey)
			} else if _, err := os.Stat(filepath.Join(dataDir, "age_key.pub")); err == nil {
				pubBytes, err := os.ReadFile(filepath.Join(dataDir, "age_key.pub"))
				if err != nil {
					return fmt.Errorf("reading existing public key: %w", err)
				}
				publicKey = string(pubBytes)
				fmt.Printf("Using existing public key: %s\n", publicKey)
			} else {
				fmt.Println("Checking local SSH keys...")
				sshPub, sshPrivPath, err := age.GetLocalSSHKey()

				useSSH := false
				if err == nil {
					prompt := &survey.Confirm{
						Message: fmt.Sprintf("Found local SSH key (%s). Use it for enbu?", sshPrivPath),
						Default: true,
					}
					_ = survey.AskOne(prompt, &useSSH)
				}

				if useSSH {
					publicKey = sshPub
					fmt.Printf("Using SSH key: %s\n", sshPrivPath)

					if err := os.MkdirAll(dataDir, 0o700); err != nil {
						return fmt.Errorf("creating data directory: %w", err)
					}
					if err := os.WriteFile(filepath.Join(dataDir, "age_key.pub"), []byte(publicKey), 0o644); err != nil {
						return fmt.Errorf("saving public key: %w", err)
					}
				} else {
					if err != nil {
						fmt.Printf("No local SSH key found: %v\n", err)
					}
					fmt.Println("Generating new age key pair...")
					kp, err := age.GenerateKeyPair()
					if err != nil {
						return fmt.Errorf("generating age key pair: %w", err)
					}
					publicKey = kp.PublicKey
					fmt.Printf("Generated age public key: %s\n", publicKey)

					encrypted, err := tokenlock.Encrypt([]byte(kp.Identity.String()), token.AccessToken)
					if err != nil {
						return fmt.Errorf("encrypting private key: %w", err)
					}

					if err := os.MkdirAll(dataDir, 0o700); err != nil {
						return fmt.Errorf("creating data directory: %w", err)
					}
					if err := os.WriteFile(filepath.Join(dataDir, "age_key.enc"), encrypted, 0o600); err != nil {
						return fmt.Errorf("saving encrypted key: %w", err)
					}
					if err := os.WriteFile(filepath.Join(dataDir, "age_key.pub"), []byte(publicKey), 0o644); err != nil {
						return fmt.Errorf("saving public key: %w", err)
					}
				}
			}

			// Push user public key
			fingerprint := keyFingerprint(publicKey)
			tag := cleanTag(fmt.Sprintf("%s-%s", token.Username, fingerprint))
			ref := fmt.Sprintf("%s:recipient-%s", registryRef, tag)
			fmt.Println("Pushing public key to registry...")
			pushOpts := &oci.PushOptions{
				SourceRepo: fmt.Sprintf("https://github.com/%s/%s", cfg.Owner, cfg.Repo),
			}
			if err := oci.Push(ctx, ref, "application/vnd.enbu.recipient.age.v1", []byte(publicKey), token.AccessToken, pushOpts); err != nil {
				return fmt.Errorf("pushing public key to GHCR: %w", err)
			}
			fmt.Println("✓ Registered user public key.")

			if joinMode {
				fmt.Println("\nYour key has been registered.")
				fmt.Println("Ask an existing member to run 'enbu sync' so you can access the secrets.")
				return nil
			}

			// Full initialization (first user)
			botRef := fmt.Sprintf("%s:recipient-github-actions", registryRef)
			fmt.Println("Generating GitHub Actions bot key...")
			botKP, err := age.GenerateKeyPair()
			if err != nil {
				return fmt.Errorf("generating bot key: %w", err)
			}

			if err := oci.Push(ctx, botRef, "application/vnd.enbu.recipient.age.v1", []byte(botKP.PublicKey), token.AccessToken, pushOpts); err != nil {
				return fmt.Errorf("pushing bot public key to GHCR: %w", err)
			}

			client := gh.NewClient(token.AccessToken)
			if err := client.CreateOrUpdateRepoSecret(ctx, cfg.Owner, cfg.Repo, "ENBU_SECRET_KEY", botKP.Identity.String()); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to register ENBU_SECRET_KEY to GitHub Secrets: %v\n", err)
				fmt.Fprintf(os.Stderr, "  You can manually add ENBU_SECRET_KEY with value:\n  %s\n", botKP.Identity.String())
			} else {
				fmt.Println("✓ Registered bot key to Repository Secrets.")
			}

			// Create enbu.toml
			projCfg := &config.ProjectConfig{Version: "0.1"}
			if err := config.SaveProject(projCfg); err != nil {
				return fmt.Errorf("creating enbu.toml: %w", err)
			}
			fmt.Println("✓ Created enbu.toml")

			// Create GitHub Actions workflow
			workflowDir := filepath.Join(repoRoot, ".github", "workflows")
			workflowPath := filepath.Join(workflowDir, "enbu-sync.yaml")
			if _, err := os.Stat(workflowPath); os.IsNotExist(err) {
				if err := os.MkdirAll(workflowDir, 0o755); err != nil {
					return fmt.Errorf("creating workflow directory: %w", err)
				}
				if err := os.WriteFile(workflowPath, templates.EnbuSyncWorkflow, 0o644); err != nil {
					return fmt.Errorf("creating workflow file: %w", err)
				}
				fmt.Println("✓ Created .github/workflows/enbu-sync.yaml")
			}

			// Create or update .gitignore
			if err := ensureGitignore(repoRoot); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to update .gitignore: %v\n", err)
			} else {
				fmt.Println("✓ Updated .gitignore")
			}

			// Auto-commit generated files
			fmt.Println("Committing generated files...")
			if err := gitCommitInitFiles(repoRoot); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: auto-commit failed: %v\n", err)
				fmt.Println("  Please commit enbu.toml and .github/workflows/enbu-sync.yaml manually.")
			} else {
				fmt.Println("✓ Committed enbu.toml and .github/workflows/enbu-sync.yaml")
			}

			fmt.Println("\nInitialization complete!")
			fmt.Println("")
			fmt.Println("Configure the package at:")
			if client.IsOrganization(ctx, cfg.Owner) {
				fmt.Printf("  https://github.com/orgs/%s/packages/container/%s-enbu/settings\n", cfg.Owner, cfg.Repo)
			} else {
				fmt.Printf("  https://github.com/users/%s/packages/container/%s-enbu/settings\n", cfg.Owner, cfg.Repo)
			}
			fmt.Println("")
			fmt.Println("  1. Manage Actions access: add this repository with Write role")
			fmt.Println("  2. Inherited access: set to \"Inherit access from source repository (recommended)\"")
			fmt.Println("")
			fmt.Println("Then:")
			fmt.Println("  3. Push the commit: git push")
			fmt.Println("  4. Run 'enbu add KEY VALUE' to add secrets")
			return nil
		},
	}
}

var gitignoreEntries = []string{
	".env",
	".env.*",
	"!.env.example",
}

func ensureGitignore(repoRoot string) error {
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

	var toAdd []string
	for _, entry := range gitignoreEntries {
		if !lineSet[entry] {
			toAdd = append(toAdd, entry)
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
	addCmd := exec.Command("git", "add", "enbu.toml", ".github/workflows/enbu-sync.yaml", ".gitignore")
	addCmd.Dir = repoRoot
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %s: %w", out, err)
	}

	commitCmd := exec.Command("git", "commit", "-m", "chore: add enbu config and sync workflow")
	commitCmd.Dir = repoRoot
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %s: %w", out, err)
	}

	return nil
}

func cleanTag(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('-')
		}
	}
	res := sb.String()
	if len(res) > 128 {
		res = res[:128]
	}
	return res
}

func keyFingerprint(pubKey string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(pubKey)))
	return hex.EncodeToString(sum[:])[:8]
}

func pullAllRecipients(ctx context.Context, ref string, token string) ([]string, error) {
	tags, err := oci.ListTags(ctx, ref, token)
	if err != nil {
		return nil, err
	}

	var publicKeys []string
	for _, tag := range tags {
		if !strings.HasPrefix(tag, "recipient-") {
			continue
		}
		tagRef := fmt.Sprintf("%s:%s", ref, tag)
		data, err := oci.Pull(ctx, tagRef, token)
		if err != nil {
			return nil, fmt.Errorf("pulling recipient %s: %w", tag, err)
		}
		publicKeys = append(publicKeys, string(data))
	}
	return publicKeys, nil
}

func secretsExists(ctx context.Context, ref, token string) bool {
	_, err := oci.GetDigest(ctx, ref, token)
	return err == nil
}

func pullSecretsWithDigest(ctx context.Context, ref, token string, identities ...agecrypto.Identity) (map[string]string, string, error) {
	digest, err := oci.GetDigest(ctx, ref, token)
	if err != nil {
		return nil, "", err
	}

	ciphertext, err := oci.Pull(ctx, ref, token)
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
