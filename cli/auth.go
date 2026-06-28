package cli

import (
	"fmt"
	"os"

	agecrypto "filippo.io/age"
	"github.com/spf13/cobra"
	"github.com/yashikota/enbu/app"
	"github.com/yashikota/enbu/auth"
	"github.com/yashikota/enbu/config"
	gh "github.com/yashikota/enbu/provider/github"
	"github.com/yashikota/enbu/utils/keystore"
)

const defaultClientID = "Ov23li6nFmfdF4FW9ikd"

func newAuthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication with GitHub",
	}

	cmd.AddCommand(
		newAuthLoginCommand(),
		newAuthLogoutCommand(),
		newAuthStatusCommand(),
	)

	return cmd
}

func newAuthLoginCommand() *cobra.Command {
	var clientID string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with GitHub via OAuth Device Flow",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			fmt.Println("Initiating GitHub authentication...")
			deviceResp, err := auth.RequestDeviceCode(ctx, clientID)
			if err != nil {
				return fmt.Errorf("requesting device code: %w", err)
			}

			if err := auth.CopyToClipboard(deviceResp.UserCode); err == nil {
				fmt.Printf("✓ Copied code to clipboard: %s\n", deviceResp.UserCode)
			} else {
				fmt.Printf("  Code: %s\n", deviceResp.UserCode)
			}

			fmt.Printf("→ Opening %s in your browser...\n", deviceResp.VerificationURI)
			if err := auth.OpenBrowser(deviceResp.VerificationURI); err != nil {
				fmt.Fprintf(os.Stderr, "  (Could not open browser automatically, visit the URL above)\n")
			}

			fmt.Println("\nWaiting for authorization...")

			tokenResp, err := auth.PollForToken(ctx, clientID, deviceResp.DeviceCode, deviceResp.Interval)
			if err != nil {
				return fmt.Errorf("polling for token: %w", err)
			}

			client := gh.NewClient(tokenResp.AccessToken)
			user, err := client.GetUser(ctx)
			if err != nil {
				return fmt.Errorf("getting user info: %w", err)
			}

			if err := auth.SaveToken(&auth.StoredToken{
				AccessToken: tokenResp.AccessToken,
				Username:    user.Login,
			}); err != nil {
				return fmt.Errorf("saving token: %w", err)
			}

			fmt.Printf("✓ Authenticated as: %s\n", user.Login)
			return nil
		},
	}

	defaultID := os.Getenv("ENBU_CLIENT_ID")
	if defaultID == "" {
		defaultID = defaultClientID
	}
	cmd.Flags().StringVar(&clientID, "client-id", defaultID, "GitHub OAuth App client ID")

	return cmd
}

func newAuthLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove stored authentication token",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := auth.DeleteToken(); err != nil {
				return err
			}

			fmt.Println("✓ Logged out successfully.")
			fmt.Println("  Note: Your age private key remains in the system keystore.")
			return nil
		},
	}
}

func newAuthStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show authentication and environment status",
		RunE: func(cmd *cobra.Command, args []string) error {
			token, err := auth.LoadToken()
			if err != nil {
				fmt.Println("Auth: not logged in")
				fmt.Println("  Run 'enbu auth login' to authenticate with GitHub")
				return nil
			}
			fmt.Printf("Auth: logged in as %s\n", token.Username)

			cfg, err := config.LoadRepo()
			if err != nil {
				fmt.Println("Repo: not in a git repository")
				return nil
			}
			fmt.Printf("Repo: %s/%s\n", cfg.Owner, cfg.Repo)

			backend, err := keystore.New()
			if err != nil {
				fmt.Printf("Keystore: error (%v)\n", err)
				return nil
			}

			repoKey := app.RepoKeystoreKey(cfg.Owner, cfg.Repo)
			privBytes, err := backend.Load(app.KeystoreService, repoKey)
			if err == nil && len(privBytes) > 0 {
				id, err := agecrypto.ParseX25519Identity(string(privBytes))
				if err == nil {
					fmt.Printf("Key: %s\n", id.Recipient().String())
				}
			} else {
				fmt.Println("Key: not initialized")
				fmt.Println("  Run 'enbu init' to generate a key pair")
			}

			if _, err := config.LoadProject(); err == nil {
				fmt.Println("Config: enbu.toml found")
			} else {
				fmt.Println("Config: not initialized")
			}

			return nil
		},
	}
}
