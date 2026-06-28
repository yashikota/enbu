//go:build scenario

package test

import (
	"os"
	"strings"
	"testing"
)

func WritePolicy(content string) Step {
	return StepFunc("write enbu.rego", func(t *testing.T, s *ScenarioState) {
		if err := os.WriteFile("enbu.rego", []byte(content), 0o644); err != nil {
			t.Fatalf("writing policy: %v", err)
		}
	})
}

func RemovePolicy() Step {
	return StepFunc("remove enbu.rego", func(t *testing.T, s *ScenarioState) {
		_ = os.Remove("enbu.rego")
	})
}

func SyncFails(user string) Step {
	return StepFunc(user+" sync fails", func(t *testing.T, s *ScenarioState) {
		u := s.user(t, user)
		if err := executeCommand(s.ctx, u.svc, "sync"); err == nil {
			t.Fatalf("expected %s sync to fail", user)
		}
	})
}

func SyncEnvFails(user, env string) Step {
	return StepFunc(user+" sync "+env+" fails", func(t *testing.T, s *ScenarioState) {
		u := s.user(t, user)
		if err := executeCommand(s.ctx, u.svc, "sync", "--env", env); err == nil {
			t.Fatalf("expected %s sync %s to fail", user, env)
		}
	})
}

func SetPlatform(user string, isOrg bool, userTeams map[string][]string, permissions map[string]string) Step {
	return StepFunc(user+" set platform", func(t *testing.T, s *ScenarioState) {
		u := s.user(t, user)
		u.svc.Platform = &mockGitHubClient{
			orgs:        map[string]bool{s.owner: isOrg},
			userTeams:   userTeams,
			permissions: permissions,
		}
	})
}

func TestScenario_PolicyAllowsAll_NoPolicyFile(t *testing.T) {
	RunScenario(t,
		Users("alice", "bob"),
		Register("alice"),
		Register("bob"),
		Add("alice", "SECRET", "value"),
		Sync("alice"),
		PullContains("bob", "value"),
	)
}

func TestScenario_PolicyDeniesRecipientByTeam(t *testing.T) {
	policy := `package enbu

import rego.v1

default allow_recipient := false

allow_recipient if {
	"infra" in input.recipient.teams
}
`
	RunScenario(t,
		StepFunc("environment config", func(t *testing.T, s *ScenarioState) {
			content := `version = "0.1"
default = "production"

[env.production]
output = ".env.production"
`
			if err := os.WriteFile("enbu.toml", []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}),
		Users("alice", "bob"),
		Register("alice"),
		Register("bob"),
		AddEnv("alice", "production", "PROD_SECRET", "classified"),

		WritePolicy(policy),
		SetPlatform("alice", true,
			map[string][]string{
				"alice": {"infra", "backend"},
				"bob":   {"backend"},
			},
			map[string]string{
				"alice": "admin",
				"bob":   "write",
			},
		),

		SyncEnv("alice", "production"),

		// alice (infra) can still decrypt
		PullEnvContainsAll("alice", "production", "PROD_SECRET", "classified"),
		// bob (backend only) cannot decrypt after policy-filtered sync
		PullFailsEnv("bob", "production"),
	)
}

func TestScenario_PolicyAllowsAllForDevEnvironment(t *testing.T) {
	policy := `package enbu

import rego.v1

default allow_recipient := false

allow_recipient if {
	input.target_env == "dev"
}

allow_recipient if {
	input.target_env == "production"
	"infra" in input.recipient.teams
}
`
	RunScenario(t,
		StepFunc("environment config", func(t *testing.T, s *ScenarioState) {
			content := `version = "0.1"
default = "dev"

[env.dev]
output = ".env.dev"

[env.production]
output = ".env.production"
`
			if err := os.WriteFile("enbu.toml", []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}),
		Users("alice", "bob"),
		Register("alice"),
		Register("bob"),
		AddEnv("alice", "dev", "DEV_KEY", "dev-value"),
		AddEnv("alice", "production", "PROD_KEY", "prod-value"),

		WritePolicy(policy),
		SetPlatform("alice", true,
			map[string][]string{
				"alice": {"infra"},
				"bob":   {"backend"},
			},
			map[string]string{
				"alice": "admin",
				"bob":   "write",
			},
		),

		// sync dev -> both allowed
		SyncEnv("alice", "dev"),
		PullEnvContainsAll("bob", "dev", "DEV_KEY", "dev-value"),

		// sync production -> only alice (infra) allowed
		SyncEnv("alice", "production"),
		PullEnvContainsAll("alice", "production", "PROD_KEY", "prod-value"),
		PullFailsEnv("bob", "production"),
	)
}

func TestScenario_PolicyDeniesAllRecipientsFails(t *testing.T) {
	policy := `package enbu

import rego.v1

default allow_recipient := false
`
	RunScenario(t,
		Users("alice", "bob"),
		Register("alice"),
		Register("bob"),
		Add("alice", "SECRET", "value"),

		WritePolicy(policy),
		SetPlatform("alice", true,
			map[string][]string{"alice": {}, "bob": {}},
			map[string]string{"alice": "admin", "bob": "write"},
		),

		// sync should fail since policy denies everyone
		SyncFails("alice"),
	)
}

func TestScenario_PolicyRemovedRestoresAccess(t *testing.T) {
	policy := `package enbu

import rego.v1

default allow_recipient := false

allow_recipient if {
	"infra" in input.recipient.teams
}
`
	RunScenario(t,
		Users("alice", "bob"),
		Register("alice"),
		Register("bob"),
		Add("alice", "SECRET", "shared-data"),

		WritePolicy(policy),
		SetPlatform("alice", true,
			map[string][]string{
				"alice": {"infra"},
				"bob":   {"backend"},
			},
			map[string]string{
				"alice": "admin",
				"bob":   "write",
			},
		),

		// bob denied by policy
		Sync("alice"),
		PullFails("bob"),

		// remove policy -> allow all again
		RemovePolicy(),
		Sync("alice"),
		PullContains("bob", "shared-data"),
	)
}

func TestScenario_PolicyPermissionBased(t *testing.T) {
	policy := `package enbu

import rego.v1

default allow_recipient := false

allow_recipient if {
	input.recipient.permission == "admin"
}
`
	RunScenario(t,
		Users("alice", "bob"),
		Register("alice"),
		Register("bob"),
		Add("alice", "SECRET", "admin-only"),

		WritePolicy(policy),
		SetPlatform("alice", false,
			nil,
			map[string]string{
				"alice": "admin",
				"bob":   "write",
			},
		),

		Sync("alice"),
		PullContains("alice", "admin-only"),
		PullFails("bob"),
	)
}

func TestScenario_PolicyInvalidRegoAborts(t *testing.T) {
	RunScenario(t,
		Users("alice", "bob"),
		Register("alice"),
		Register("bob"),
		Add("alice", "KEY", "value"),

		WritePolicy("this is not valid rego {{{"),
		SetPlatform("alice", true, nil, nil),

		SyncFails("alice"),
	)
}

func TestScenario_PolicyMultiTeamAccess(t *testing.T) {
	policy := `package enbu

import rego.v1

default allow_recipient := false

allow_recipient if {
	input.target_env == "staging"
	"backend" in input.recipient.teams
}

allow_recipient if {
	input.target_env == "staging"
	"frontend" in input.recipient.teams
}
`
	RunScenario(t,
		StepFunc("environment config", func(t *testing.T, s *ScenarioState) {
			content := `version = "0.1"
default = "staging"

[env.staging]
output = ".env.staging"
`
			if err := os.WriteFile("enbu.toml", []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}),
		Users("alice", "bob", "charlie"),
		Register("alice"),
		Register("bob"),
		Register("charlie"),
		AddEnv("alice", "staging", "STAGING_KEY", "staging-val"),

		WritePolicy(policy),
		SetPlatform("alice", true,
			map[string][]string{
				"alice":   {"backend"},
				"bob":     {"frontend"},
				"charlie": {"design"},
			},
			map[string]string{
				"alice":   "write",
				"bob":     "write",
				"charlie": "write",
			},
		),

		SyncEnv("alice", "staging"),
		// alice (backend) and bob (frontend) allowed
		PullEnvContainsAll("alice", "staging", "STAGING_KEY"),
		PullEnvContainsAll("bob", "staging", "STAGING_KEY"),
		// charlie (design only) denied
		PullFailsEnv("charlie", "staging"),
	)
}

func TestScenario_PolicySyncPreservesSecretContent(t *testing.T) {
	policy := `package enbu

import rego.v1

default allow_recipient := false

allow_recipient if {
	"infra" in input.recipient.teams
}
`
	RunScenario(t,
		Users("alice", "bob"),
		Register("alice"),
		Register("bob"),
		Add("alice", "DB_URL", "postgres://prod:5432/db"),
		Add("alice", "API_KEY", "sk-live-secret"),
		Add("alice", "REDIS", "redis://cache:6379"),

		WritePolicy(policy),
		SetPlatform("alice", true,
			map[string][]string{
				"alice": {"infra"},
				"bob":   {"backend"},
			},
			map[string]string{
				"alice": "admin",
				"bob":   "write",
			},
		),

		Sync("alice"),
		// alice can still see all secrets after filtered sync
		StepFunc("alice sees all secrets intact", func(t *testing.T, s *ScenarioState) {
			output := pullStdout(t, s.ctx, s.user(t, "alice"))
			for _, want := range []string{"DB_URL", "postgres://prod:5432/db", "API_KEY", "sk-live-secret", "REDIS", "redis://cache:6379"} {
				if !strings.Contains(output, want) {
					t.Fatalf("missing %q in output: %s", want, output)
				}
			}
		}),
	)
}
