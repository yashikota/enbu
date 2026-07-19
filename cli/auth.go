package cli

import (
	"fmt"

	agecrypto "filippo.io/age"
	"github.com/enbu-net/enbu/app"
	"github.com/enbu-net/enbu/auth"
	"github.com/enbu-net/enbu/config"
	"github.com/enbu-net/enbu/utils/keystore"
	"github.com/spf13/cobra"
)

func newAuthCommand(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication with GitHub",
	}

	cmd.AddCommand(
		newAuthLoginCommand(),
		newAuthLogoutCommand(),
		newAuthStatusCommand(a),
	)

	return cmd
}

func newAuthLoginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with GitHub",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			fmt.Println("Initiating GitHub authentication...")
			token, err := auth.Login(ctx, func(authorizeURL string) error {
				fmt.Println("→ Opening GitHub in your browser...")
				fmt.Println("Waiting for authorization...")
				return auth.OpenBrowser(authorizeURL)
			})
			if err != nil {
				return err
			}

			fmt.Printf("✓ Authenticated as: %s\n", token.Username)
			return nil
		},
	}

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

func newAuthStatusCommand(a *app.App) *cobra.Command {
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

			owner, repo, err := a.RepoDetector.LoadRepo()
			if err != nil {
				fmt.Println("Repo: not in a git repository")
				return nil
			}
			fmt.Printf("Repo: %s/%s\n", owner, repo)

			backend, err := keystore.New()
			if err != nil {
				fmt.Printf("Keystore: error (%v)\n", err)
				return nil
			}

			repoKey := app.RepoKeystoreKey(owner, repo)
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
