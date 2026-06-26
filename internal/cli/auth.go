package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/yashikota/enbu/internal/auth"
	"github.com/yashikota/enbu/internal/config"
	gh "github.com/yashikota/enbu/internal/github"
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
	var force bool

	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove stored authentication token",
		RunE: func(cmd *cobra.Command, args []string) error {
			dataDir := config.DataDir()
			encKeyPath := filepath.Join(dataDir, "age_key.enc")

			if !force {
				if _, err := os.Stat(encKeyPath); err == nil {
					fmt.Println("Warning: Your age private key is encrypted with your OAuth token.")
					fmt.Println("         After logout, you won't be able to decrypt it until you log in again.")

					var confirm bool
					prompt := &survey.Confirm{
						Message: "Continue with logout?",
						Default: false,
					}
					if err := survey.AskOne(prompt, &confirm); err != nil {
						return err
					}
					if !confirm {
						fmt.Println("Logout cancelled.")
						return nil
					}
				}
			}

			if err := auth.DeleteToken(); err != nil {
				return err
			}

			fmt.Println("✓ Logged out successfully.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	return cmd
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

			dataDir := config.DataDir()
			if _, err := os.Stat(filepath.Join(dataDir, "age_key.enc")); err == nil {
				pubBytes, _ := os.ReadFile(filepath.Join(dataDir, "age_key.pub"))
				fmt.Printf("Key (age): %s\n", string(pubBytes))
			} else if _, err := os.Stat(filepath.Join(dataDir, "age_key.pub")); err == nil {
				pubBytes, _ := os.ReadFile(filepath.Join(dataDir, "age_key.pub"))
				fmt.Printf("Key (SSH): %s\n", string(pubBytes))
			} else {
				fmt.Println("Key:  not initialized")
				fmt.Println("  Run 'enbu init' to generate a key pair")
			}

			cfg, err := config.LoadRepo()
			if err != nil {
				fmt.Println("Repo: not in a git repository")
			} else {
				fmt.Printf("Repo: %s/%s\n", cfg.Owner, cfg.Repo)
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
