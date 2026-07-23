package cli

import (
	"context"
	"fmt"
	"os"

	agecrypto "filippo.io/age"
	"github.com/enbu-net/enbu/app"
	"github.com/enbu-net/enbu/auth"
	"github.com/enbu-net/enbu/config"
	"github.com/enbu-net/enbu/utils/keystore"
	"github.com/spf13/cobra"
)

const defaultDeviceClientID = "Ov23li6nFmfdF4FW9ikd"

type authLoginDeps struct {
	browserLogin func(context.Context, auth.BrowserOpener) (*auth.StoredToken, error)
	deviceLogin  func(context.Context, string, auth.DevicePrompter) (*auth.StoredToken, error)
	openBrowser  auth.BrowserOpener
}

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
	return newAuthLoginCommandWithDeps(authLoginDeps{
		browserLogin: auth.Login,
		deviceLogin:  auth.LoginDevice,
		openBrowser:  auth.OpenBrowser,
	})
}

func newAuthLoginCommandWithDeps(deps authLoginDeps) *cobra.Command {
	var deviceFlow bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with GitHub",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cmd.Println("Initiating GitHub authentication...")
			var token *auth.StoredToken
			var err error
			if deviceFlow {
				clientID := os.Getenv("ENBU_CLIENT_ID")
				if clientID == "" {
					clientID = defaultDeviceClientID
				}
				token, err = deps.deviceLogin(ctx, clientID, func(device auth.DeviceAuthorization) error {
					cmd.Printf("Code: %s\n", device.UserCode)
					cmd.Printf("→ Opening %s in your browser...\n", device.VerificationURI)
					if err := deps.openBrowser(device.VerificationURI); err != nil {
						cmd.PrintErrln("Could not open the browser automatically; open the URL above manually.")
					}
					cmd.Println("Waiting for authorization...")
					return nil
				})
			} else {
				token, err = deps.browserLogin(ctx, func(authorizeURL string) error {
					cmd.Println("→ Opening GitHub in your browser...")
					cmd.Println("Waiting for authorization...")
					return deps.openBrowser(authorizeURL)
				})
			}
			if err != nil {
				return err
			}

			cmd.Printf("✓ Authenticated as: %s\n", token.Username)
			return nil
		},
	}
	cmd.Flags().BoolVar(&deviceFlow, "device", false, "Authenticate with GitHub Device Flow")

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
