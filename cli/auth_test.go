package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/yashikota/enbu/auth"
)

func setupCLIAuthTest(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_ACTOR", "")
}

func TestAuthSwitchUsesUserFlag(t *testing.T) {
	setupCLIAuthTest(t)
	writeCLIAuthStore(t, twoAccountStore())
	command := newAuthSwitchCommand()
	command.SetArgs([]string{"--user", "Bob"})
	output, err := executeAuthCommand(command)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "Switched active account for github.com to Bob") {
		t.Fatalf("output = %q", output)
	}
	token, err := auth.LoadToken()
	if err != nil || token.Username != "Bob" {
		t.Fatalf("LoadToken() = %#v, %v", token, err)
	}
}

func TestAuthSwitchAutomaticallySelectsInactiveUserWhenTwoAccountsExist(t *testing.T) {
	setupCLIAuthTest(t)
	writeCLIAuthStore(t, twoAccountStore())
	output, err := executeAuthCommand(newAuthSwitchCommand())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "to Bob") {
		t.Fatalf("output = %q", output)
	}
}

func TestAuthSwitchRequiresUserWithoutPromptForMoreThanTwoAccounts(t *testing.T) {
	setupCLIAuthTest(t)
	writeCLIAuthStore(t, threeAccountStore())
	_, err := executeAuthCommand(newAuthSwitchCommand())
	if err == nil || !strings.Contains(err.Error(), "please specify `--hostname` and `--user`") {
		t.Fatalf("error = %v", err)
	}
}

func TestAuthSwitchPromptsForMoreThanTwoAccounts(t *testing.T) {
	setupCLIAuthTest(t)
	writeCLIAuthStore(t, threeAccountStore())
	previous := authCanPrompt
	authCanPrompt = func(*cobra.Command) bool { return true }
	t.Cleanup(func() { authCanPrompt = previous })
	command := newAuthSwitchCommand()
	command.SetIn(strings.NewReader("3\n"))
	output, err := executeAuthCommand(command)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "What account do you want to switch to?") || !strings.Contains(output, "to Charlie") {
		t.Fatalf("output = %q", output)
	}
}

func TestAuthLogoutUsesUserFlagAndSwitchesAfterRemovingActiveAccount(t *testing.T) {
	setupCLIAuthTest(t)
	writeCLIAuthStore(t, twoAccountStore())
	command := newAuthLogoutCommand()
	command.SetArgs([]string{"-u", "Alice"})
	output, err := executeAuthCommand(command)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "Logged out of github.com account Alice") || !strings.Contains(output, "Switched active account for github.com to Bob") {
		t.Fatalf("output = %q", output)
	}
	accounts, err := auth.ListAccounts()
	if err != nil || len(accounts) != 1 || accounts[0].Username != "Bob" || !accounts[0].Active {
		t.Fatalf("ListAccounts() = %#v, %v", accounts, err)
	}
}

func TestAuthLogoutRequiresUserWithoutPromptForMultipleAccounts(t *testing.T) {
	setupCLIAuthTest(t)
	writeCLIAuthStore(t, twoAccountStore())
	_, err := executeAuthCommand(newAuthLogoutCommand())
	if err == nil || !strings.Contains(err.Error(), "please specify `--hostname` and `--user`") {
		t.Fatalf("error = %v", err)
	}
}

func TestAuthStatusDisplaysAllAccountsAndActiveFilter(t *testing.T) {
	setupCLIAuthTest(t)
	writeCLIAuthStore(t, twoAccountStore())
	output, err := executeAuthCommand(newAuthStatusCommand())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"github.com", "account Alice (file)", "account Bob (file)", "Active account: true", "alic_"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output %q does not contain %q", output, want)
		}
	}

	activeCommand := newAuthStatusCommand()
	activeCommand.SetArgs([]string{"--active", "--show-token"})
	activeOutput, err := executeAuthCommand(activeCommand)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(activeOutput, "alice-token") || strings.Contains(activeOutput, "account Bob") {
		t.Fatalf("active output = %q", activeOutput)
	}
}

func TestEnvironmentTokenOverridesStoredAccountAndBlocksMutation(t *testing.T) {
	setupCLIAuthTest(t)
	writeCLIAuthStore(t, twoAccountStore())
	t.Setenv("GH_TOKEN", "environment-token")
	t.Setenv("GITHUB_ACTOR", "actions-user")

	statusOutput, err := executeAuthCommand(newAuthStatusCommand())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(statusOutput, "account actions-user (GH_TOKEN)") || !strings.Contains(statusOutput, "Active account: true") {
		t.Fatalf("status output = %q", statusOutput)
	}

	_, switchErr := executeAuthCommand(newAuthSwitchCommand())
	if switchErr == nil || !strings.Contains(switchErr.Error(), "GH_TOKEN") {
		t.Fatalf("switch error = %v", switchErr)
	}
	logout := newAuthLogoutCommand()
	logout.SetArgs([]string{"--user", "Alice"})
	_, logoutErr := executeAuthCommand(logout)
	if logoutErr == nil || !strings.Contains(logoutErr.Error(), "GH_TOKEN") {
		t.Fatalf("logout error = %v", logoutErr)
	}
}

func TestAuthLoginRefusesToStoreCredentialsWhileEnvironmentTokenIsSet(t *testing.T) {
	setupCLIAuthTest(t)
	t.Setenv("GH_TOKEN", "environment-token")
	_, err := executeAuthCommand(newAuthLoginCommand())
	if err == nil || !strings.Contains(err.Error(), "GH_TOKEN") {
		t.Fatalf("login error = %v", err)
	}
}

func TestAuthCommandsRejectUnsupportedHostname(t *testing.T) {
	setupCLIAuthTest(t)
	command := newAuthSwitchCommand()
	command.SetArgs([]string{"--hostname", "enterprise.internal", "--user", "Alice"})
	_, err := executeAuthCommand(command)
	if err == nil || !strings.Contains(err.Error(), "not logged in to enterprise.internal") {
		t.Fatalf("hostname error = %v", err)
	}
}

func TestAuthCommandsRejectLegacyPositionArguments(t *testing.T) {
	setupCLIAuthTest(t)
	writeCLIAuthStore(t, twoAccountStore())
	switchCommand := newAuthSwitchCommand()
	switchCommand.SetArgs([]string{"Bob"})
	if _, err := executeAuthCommand(switchCommand); err == nil {
		t.Fatal("auth switch accepted a positional username")
	}
	logoutCommand := newAuthLogoutCommand()
	logoutCommand.SetArgs([]string{"Bob"})
	if _, err := executeAuthCommand(logoutCommand); err == nil {
		t.Fatal("auth logout accepted a positional username")
	}
}

func executeAuthCommand(command *cobra.Command) (string, error) {
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	err := command.Execute()
	return output.String(), err
}

func writeCLIAuthStore(t *testing.T, contents string) {
	t.Helper()
	path := auth.TokenPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func twoAccountStore() string {
	return `{"version":2,"active":"alice","accounts":{"alice":{"username":"Alice","storage":"file","access_token":"alice-token"},"bob":{"username":"Bob","storage":"file","access_token":"bob-token"}}}`
}

func threeAccountStore() string {
	return `{"version":2,"active":"alice","accounts":{"alice":{"username":"Alice","storage":"file","access_token":"alice-token"},"bob":{"username":"Bob","storage":"file","access_token":"bob-token"},"charlie":{"username":"Charlie","storage":"file","access_token":"charlie-token"}}}`
}
