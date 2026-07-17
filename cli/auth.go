package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
	"github.com/yashikota/enbu/auth"
	gh "github.com/yashikota/enbu/provider/github"
)

const (
	defaultClientID = "Ov23li6nFmfdF4FW9ikd"
	githubHostname  = "github.com"
)

var authCanPrompt = func(cmd *cobra.Command) bool {
	input, ok := cmd.InOrStdin().(*os.File)
	return ok && term.IsTerminal(input.Fd())
}

func DefaultClientID() string {
	return defaultClientID
}

func newAuthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate enbu with GitHub",
	}

	cmd.AddCommand(
		newAuthLoginCommand(),
		newAuthLogoutCommand(),
		newAuthStatusCommand(),
		newAuthSwitchCommand(),
	)

	return cmd
}

func newAuthLoginCommand() *cobra.Command {
	var clientID string
	var hostname string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to a GitHub account",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateGitHubHostname(hostname); err != nil {
				return err
			}
			if source := auth.EnvironmentTokenSource(); source != "" {
				return fmt.Errorf("the value of the %s environment variable is being used for authentication; clear it before storing credentials", source)
			}

			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			if _, err := fmt.Fprintln(out, "Initiating GitHub authentication..."); err != nil {
				return err
			}
			deviceResp, err := auth.RequestDeviceCode(ctx, clientID)
			if err != nil {
				return fmt.Errorf("requesting device code: %w", err)
			}

			if err := auth.CopyToClipboard(deviceResp.UserCode); err == nil {
				if _, err := fmt.Fprintf(out, "✓ Copied code to clipboard: %s\n", deviceResp.UserCode); err != nil {
					return err
				}
			} else if _, err := fmt.Fprintf(out, "  Code: %s\n", deviceResp.UserCode); err != nil {
				return err
			}

			if _, err := fmt.Fprintf(out, "→ Opening %s in your browser...\n", deviceResp.VerificationURI); err != nil {
				return err
			}
			if err := auth.OpenBrowser(deviceResp.VerificationURI); err != nil {
				if _, writeErr := fmt.Fprintln(cmd.ErrOrStderr(), "  (Could not open browser automatically, visit the URL above)"); writeErr != nil {
					return writeErr
				}
			}

			if _, err := fmt.Fprintln(out, "\nWaiting for authorization..."); err != nil {
				return err
			}

			tokenResp, err := auth.PollForToken(ctx, clientID, deviceResp.DeviceCode, deviceResp.Interval)
			if err != nil {
				return fmt.Errorf("polling for token: %w", err)
			}

			client := gh.NewClient(tokenResp.AccessToken)
			user, err := client.GetUser(ctx)
			if err != nil {
				return fmt.Errorf("getting user info: %w", err)
			}

			account, err := auth.SaveTokenWithAccount(&auth.StoredToken{
				AccessToken: tokenResp.AccessToken,
				Username:    user.Login,
			})
			if err != nil {
				return fmt.Errorf("saving token: %w", err)
			}

			commandPrintf(cmd.ErrOrStderr(), "✓ Logged in to %s account %s\n", githubHostname, user.Login)
			if account.Storage == "file" {
				commandPrintf(cmd.ErrOrStderr(), "! Authentication credentials saved in plain text at %s\n", auth.TokenPath())
			}
			return nil
		},
	}

	defaultID := os.Getenv("ENBU_CLIENT_ID")
	if defaultID == "" {
		defaultID = defaultClientID
	}
	cmd.Flags().Bool("help", false, "help for login")
	cmd.Flags().StringVar(&clientID, "client-id", defaultID, "GitHub OAuth App client ID")
	cmd.Flags().StringVarP(&hostname, "hostname", "h", githubHostname, "The hostname of the GitHub instance to authenticate with")

	return cmd
}

func newAuthSwitchCommand() *cobra.Command {
	var hostname string
	var username string

	cmd := &cobra.Command{
		Use:   "switch",
		Short: "Switch active GitHub account",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateGitHubHostname(hostname); err != nil {
				return err
			}
			if source := auth.EnvironmentTokenSource(); source != "" {
				return fmt.Errorf("the value of the %s environment variable is being used for authentication; clear it before switching accounts", source)
			}

			accounts, err := storedAccounts()
			if err != nil {
				return err
			}
			if len(accounts) == 0 {
				return fmt.Errorf("not logged in to any hosts")
			}

			account, err := switchCandidate(cmd, accounts, username)
			if err != nil {
				return err
			}
			if err := auth.SwitchAccount(account.ID); err != nil {
				return err
			}
			commandPrintf(cmd.ErrOrStderr(), "✓ Switched active account for %s to %s\n", githubHostname, account.Username)
			return nil
		},
	}
	cmd.Flags().Bool("help", false, "help for switch")
	cmd.Flags().StringVarP(&hostname, "hostname", "h", githubHostname, "The hostname of the GitHub instance to switch account for")
	cmd.Flags().StringVarP(&username, "user", "u", "", "The account to switch to")
	return cmd
}

func newAuthLogoutCommand() *cobra.Command {
	var hostname string
	var username string

	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Log out of a GitHub account",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateGitHubHostname(hostname); err != nil {
				return err
			}
			if source := auth.EnvironmentTokenSource(); source != "" {
				return fmt.Errorf("the value of the %s environment variable is being used for authentication; clear it before erasing stored credentials", source)
			}

			accounts, err := storedAccounts()
			if err != nil {
				return err
			}
			if len(accounts) == 0 {
				return fmt.Errorf("not logged in to any hosts")
			}
			account, err := logoutCandidate(cmd, accounts, username)
			if err != nil {
				return err
			}
			wasActive := account.Active
			if err := auth.RemoveAccount(account.ID); err != nil {
				return err
			}
			commandPrintf(cmd.ErrOrStderr(), "✓ Logged out of %s account %s\n", githubHostname, account.Username)
			if wasActive {
				remaining, err := storedAccounts()
				if err != nil {
					return err
				}
				for _, candidate := range remaining {
					if candidate.Active {
						commandPrintf(cmd.ErrOrStderr(), "✓ Switched active account for %s to %s\n", githubHostname, candidate.Username)
						break
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().Bool("help", false, "help for logout")
	cmd.Flags().StringVarP(&hostname, "hostname", "h", githubHostname, "The hostname of the GitHub instance to log out of")
	cmd.Flags().StringVarP(&username, "user", "u", "", "The account to log out of")
	return cmd
}

func newAuthStatusCommand() *cobra.Command {
	var activeOnly bool
	var hostname string
	var showToken bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "View authentication status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateGitHubHostname(hostname); err != nil {
				return err
			}
			accounts, err := auth.ListAccounts()
			if err != nil {
				return err
			}
			if len(accounts) == 0 {
				return fmt.Errorf("not logged in to any hosts")
			}

			commandPrintln(cmd.OutOrStdout(), githubHostname)
			for _, account := range accounts {
				if activeOnly && !account.Active {
					continue
				}
				storage := account.Storage
				if account.Source == "environment" {
					storage = auth.EnvironmentTokenSource()
				}
				commandPrintf(cmd.OutOrStdout(), "  ✓ Logged in to %s account %s (%s)\n", githubHostname, account.Username, storage)
				commandPrintf(cmd.OutOrStdout(), "  - Active account: %t\n", account.Active)
				token, loadErr := tokenForStatusAccount(account)
				if loadErr == nil {
					if showToken {
						commandPrintf(cmd.OutOrStdout(), "  - Token: %s\n", token.AccessToken)
					} else {
						commandPrintf(cmd.OutOrStdout(), "  - Token: %s\n", maskedToken(token.AccessToken))
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().Bool("help", false, "help for status")
	cmd.Flags().BoolVarP(&activeOnly, "active", "a", false, "Display the active account only")
	cmd.Flags().StringVarP(&hostname, "hostname", "h", githubHostname, "Check only a specific hostname's auth status")
	cmd.Flags().BoolVarP(&showToken, "show-token", "t", false, "Display the authentication token")
	return cmd
}

func switchCandidate(cmd *cobra.Command, accounts []auth.Account, username string) (auth.Account, error) {
	if username != "" {
		return matchingAccount(accounts, username)
	}
	if len(accounts) == 1 {
		return accounts[0], nil
	}
	if len(accounts) == 2 {
		if !accounts[0].Active {
			return accounts[0], nil
		}
		return accounts[1], nil
	}
	if !authCanPrompt(cmd) {
		return auth.Account{}, fmt.Errorf("unable to determine which account to switch to, please specify `--hostname` and `--user`")
	}
	return promptForAccount(cmd, "What account do you want to switch to?", accounts, true)
}

func logoutCandidate(cmd *cobra.Command, accounts []auth.Account, username string) (auth.Account, error) {
	if username != "" {
		return matchingAccount(accounts, username)
	}
	if len(accounts) == 1 {
		return accounts[0], nil
	}
	if !authCanPrompt(cmd) {
		return auth.Account{}, fmt.Errorf("unable to determine which account to log out of, please specify `--hostname` and `--user`")
	}
	return promptForAccount(cmd, "What account do you want to log out of?", accounts, false)
}

func matchingAccount(accounts []auth.Account, username string) (auth.Account, error) {
	for _, account := range accounts {
		if strings.EqualFold(account.Username, username) {
			return account, nil
		}
	}
	return auth.Account{}, fmt.Errorf("not logged in to %s account %s", githubHostname, username)
}

func promptForAccount(cmd *cobra.Command, question string, accounts []auth.Account, markActive bool) (auth.Account, error) {
	commandPrintf(cmd.ErrOrStderr(), "? %s\n", question)
	for index, account := range accounts {
		suffix := ""
		if markActive && account.Active {
			suffix = " - active"
		}
		commandPrintf(cmd.ErrOrStderr(), "  %d. %s (%s)%s\n", index+1, account.Username, githubHostname, suffix)
	}
	commandPrintf(cmd.ErrOrStderr(), "Select an account [1-%d]: ", len(accounts))
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" {
		return auth.Account{}, fmt.Errorf("could not prompt: %w", err)
	}
	selection, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || selection < 1 || selection > len(accounts) {
		return auth.Account{}, fmt.Errorf("invalid account selection")
	}
	return accounts[selection-1], nil
}

func storedAccounts() ([]auth.Account, error) {
	accounts, err := auth.ListAccounts()
	if err != nil {
		return nil, err
	}
	stored := make([]auth.Account, 0, len(accounts))
	for _, account := range accounts {
		if account.Source == "stored" {
			stored = append(stored, account)
		}
	}
	return stored, nil
}

func tokenForStatusAccount(account auth.Account) (*auth.StoredToken, error) {
	if account.Source == "environment" {
		return auth.LoadToken()
	}
	return auth.LoadTokenFor(account.Username)
}

func maskedToken(token string) string {
	if len(token) <= 4 {
		return strings.Repeat("*", len(token))
	}
	return token[:4] + "_" + strings.Repeat("*", 36)
}

func validateGitHubHostname(hostname string) error {
	if hostname != githubHostname {
		return fmt.Errorf("not logged in to %s", hostname)
	}
	return nil
}

func commandPrintln(writer io.Writer, values ...any) {
	_, _ = fmt.Fprintln(writer, values...)
}

func commandPrintf(writer io.Writer, format string, values ...any) {
	_, _ = fmt.Fprintf(writer, format, values...)
}
