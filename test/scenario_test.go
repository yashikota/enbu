//go:build scenario

package test

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestScenario_SingleUserAddPull(t *testing.T) {
	RunScenario(t,
		Users("alice"),
		Register("alice"),
		Add("alice", "DB_HOST", "localhost"),
		Add("alice", "DB_PORT", "5432"),
		PullContainsAll("alice", "DB_HOST=", "localhost", "DB_PORT=", "5432"),
	)
}

func TestScenario_JoinFlowRequiresSync(t *testing.T) {
	RunScenario(t,
		Users("alice", "bob"),
		Register("alice"),
		Add("alice", "SECRET", "only-for-alice"),
		Register("bob"),
		PullFails("bob"),
		Sync("alice"),
		PullContains("bob", "only-for-alice"),
	)
}

func TestScenario_ThreeUsersSequentialJoin(t *testing.T) {
	RunScenario(t,
		Users("alice", "bob", "charlie"),
		Register("alice"),
		Add("alice", "SHARED_KEY", "initial-value"),
		Register("bob"),
		PullFails("bob"),
		Sync("alice"),
		PullContains("bob", "initial-value"),
		Add("alice", "NEW_KEY", "after-bob-joined"),
		PullContains("bob", "after-bob-joined"),
		Register("charlie"),
		PullFails("charlie"),
		Sync("bob"),
		PullContainsAll("charlie", "initial-value", "after-bob-joined"),
	)
}

func TestScenario_AddRejectsExistingSecretAndEditUpdates(t *testing.T) {
	RunScenario(t,
		Users("alice"),
		Register("alice"),
		Add("alice", "API_KEY", "old-key"),
		AddFails("alice", "API_KEY", "new-key"),
		PullContains("alice", "old-key"),
		Edit("alice", "API_KEY", "new-key"),
		PullDoesNotContain("alice", "old-key"),
		PullContains("alice", "new-key"),
	)
}

func TestScenario_DeleteSecret(t *testing.T) {
	RunScenario(t,
		Users("alice"),
		Register("alice"),
		Add("alice", "API_KEY", "sk-secret"),
		Add("alice", "DATABASE_URL", "postgres://prod/app"),
		Delete("alice", "API_KEY"),
		PullDoesNotContain("alice", "API_KEY=", "sk-secret"),
		PullContainsAll("alice", "DATABASE_URL=", "postgres://prod/app"),
	)
}

func TestScenario_DeleteMissingSecretIsNoop(t *testing.T) {
	RunScenario(t,
		Users("alice"),
		Register("alice"),
		Add("alice", "DATABASE_URL", "postgres://prod/app"),
		Delete("alice", "MISSING_KEY"),
		PullContainsAll("alice", "DATABASE_URL=", "postgres://prod/app"),
	)
}

func TestScenario_DeleteBeforeFirstSecretIsNoop(t *testing.T) {
	RunScenario(t,
		Users("alice"),
		Register("alice"),
		Delete("alice", "MISSING_KEY"),
		PullFails("alice"),
	)
}

func TestScenario_SpecialCharacterValues(t *testing.T) {
	testCases := []struct{ key, value string }{
		{"JAPANESE", "日本語のシークレット"},
		{"EMOJI", "🔑🔒✨"},
		{"NEWLINES", "line1\\nline2\\nline3"},
		{"QUOTES", `he said "hello"`},
		{"EQUALS", "key=value=extra"},
		{"SPACES", "  leading and trailing  "},
		{"EMPTY", ""},
		{"URL", "postgres://user:p@ss@host:5432/db?sslmode=require"},
	}

	steps := []Step{
		Users("alice"),
		Register("alice"),
	}
	for _, tc := range testCases {
		steps = append(steps, Add("alice", tc.key, tc.value))
	}
	for _, tc := range testCases {
		steps = append(steps, PullContains("alice", tc.key+"="))
	}
	steps = append(steps,
		PullContains("alice", "日本語のシークレット"),
		PullContains("alice", "🔑🔒✨"),
	)

	RunScenario(t, steps...)
}

func TestScenario_ManySecrets(t *testing.T) {
	steps := []Step{
		Users("alice"),
		Register("alice"),
	}

	const count = 50
	for i := range count {
		steps = append(steps, Add("alice", fmt.Sprintf("SECRET_%03d", i), fmt.Sprintf("value-%d", i)))
	}

	steps = append(steps, StepFunc("alice sees all added secret keys", func(t *testing.T, s *ScenarioState) {
		output := pullStdout(t, s.ctx, s.user(t, "alice"))
		for i := range count {
			key := fmt.Sprintf("SECRET_%03d", i)
			if !strings.Contains(output, key+"=") {
				t.Fatalf("missing %s after adding %d secrets", key, count)
			}
		}
	}))

	RunScenario(t, steps...)
}

func TestScenario_PullNoSecrets(t *testing.T) {
	RunScenario(t,
		Users("alice"),
		Register("alice"),
		PullFails("alice"),
	)
}

func TestScenario_SyncIdempotent(t *testing.T) {
	RunScenario(t,
		Users("alice", "bob"),
		Register("alice"),
		Register("bob"),
		Add("alice", "KEY", "value"),
		Sync("alice"),
		Sync("alice"),
		Sync("bob"),
		PullContains("alice", "value"),
		PullContains("bob", "value"),
	)
}

func TestScenario_ConcurrentAdds(t *testing.T) {
	RunScenario(t,
		Users("alice", "bob"),
		Register("alice"),
		Register("bob"),
		Add("alice", "SEED", "initial"),
		StepFunc("alice and bob add concurrently", func(t *testing.T, s *ScenarioState) {
			user1 := s.user(t, "alice")
			user2 := s.user(t, "bob")

			var wg sync.WaitGroup
			var err1, err2 error

			wg.Add(2)
			go func() {
				defer wg.Done()
				err1 = executeCommand(s.ctx, user1.svc, "add", "FROM_ALICE", "alice-data")
			}()
			go func() {
				defer wg.Done()
				err2 = executeCommand(s.ctx, user2.svc, "add", "FROM_BOB", "bob-data")
			}()
			wg.Wait()

			if err1 != nil && err2 != nil {
				t.Fatalf("both adds failed: err1=%v, err2=%v", err1, err2)
			}

			if err1 != nil {
				addSecret(t, s.ctx, user1, "FROM_ALICE", "alice-data")
			}
			if err2 != nil {
				addSecret(t, s.ctx, user2, "FROM_BOB", "bob-data")
			}
		}),
		StepFunc("at least one concurrent add survives", func(t *testing.T, s *ScenarioState) {
			output := pullStdout(t, s.ctx, s.user(t, "alice"))
			if !strings.Contains(output, "SEED") {
				t.Fatalf("missing SEED: %s", output)
			}
			if !strings.Contains(output, "FROM_ALICE") || !strings.Contains(output, "FROM_BOB") {
				output2 := pullStdout(t, s.ctx, s.user(t, "bob"))
				t.Logf("alice sees: %s", output)
				t.Logf("bob sees: %s", output2)
				if !strings.Contains(output, "FROM_ALICE") && !strings.Contains(output, "FROM_BOB") {
					t.Fatal("neither concurrent add survived")
				}
			}
		}),
	)
}

func TestScenario_AddAfterSyncPreservesRecipients(t *testing.T) {
	RunScenario(t,
		Users("alice", "bob"),
		Register("alice"),
		Add("alice", "FIRST", "first-value"),
		Register("bob"),
		Sync("alice"),
		Add("bob", "SECOND", "second-value"),
		PullContainsAll("alice", "first-value", "second-value"),
		PullContainsAll("bob", "first-value", "second-value"),
	)
}

func TestScenario_NewRecipientCannotAddUntilSynced(t *testing.T) {
	RunScenario(t,
		Users("alice", "bob"),
		Register("alice"),
		Add("alice", "DATABASE_URL", "postgres://prod/app"),
		Register("bob"),
		AddFails("bob", "BOB_ONLY", "not-yet"),
		Sync("alice"),
		Add("bob", "BOB_ONLY", "after-sync"),
		PullContainsAll("alice", "DATABASE_URL", "BOB_ONLY", "after-sync"),
		PullContainsAll("bob", "DATABASE_URL", "BOB_ONLY", "after-sync"),
	)
}

func TestScenario_SyncBeforeFirstSecretThenTeamCanPull(t *testing.T) {
	RunScenario(t,
		Users("alice", "bob"),
		Register("alice"),
		Register("bob"),
		Sync("alice"),
		Add("alice", "FIRST_SECRET", "created-after-empty-sync"),
		PullContains("bob", "created-after-empty-sync"),
	)
}

func TestScenario_EnvironmentSecretsAreIsolated(t *testing.T) {
	RunScenario(t,
		StepFunc("environment config exists", func(t *testing.T, s *ScenarioState) {
			content := `version = "0.1"
default = "dev"

[env.dev]
output = ".env.dev"

[env.prod]
output = ".env.prod"
`
			if err := os.WriteFile("enbu.toml", []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}),
		Users("alice", "bob"),
		Register("alice"),
		Register("bob"),
		AddEnv("alice", "dev", "API_KEY", "dev-secret"),
		AddEnv("alice", "prod", "API_KEY", "prod-secret"),
		PullEnvContainsAll("alice", "dev", "API_KEY", "dev-secret"),
		PullEnvDoesNotContain("alice", "dev", "prod-secret"),
		PullEnvContainsAll("alice", "prod", "API_KEY", "prod-secret"),
		PullEnvDoesNotContain("alice", "prod", "dev-secret"),
		PullEnvContainsAll("bob", "dev", "dev-secret"),
		PullEnvContainsAll("bob", "prod", "prod-secret"),
	)
}

func TestScenario_RotateSharedSecretForWholeTeam(t *testing.T) {
	RunScenario(t,
		Users("alice", "bob", "charlie"),
		Register("alice"),
		Register("bob"),
		Add("alice", "API_TOKEN", "old-token"),
		Add("alice", "UNCHANGED", "still-here"),
		Sync("alice"),
		Register("charlie"),
		Sync("bob"),
		Edit("charlie", "API_TOKEN", "new-token"),
		PullDoesNotContain("alice", "old-token"),
		PullContainsAll("alice", "API_TOKEN", "new-token", "UNCHANGED", "still-here"),
		PullDoesNotContain("bob", "old-token"),
		PullContainsAll("bob", "API_TOKEN", "new-token", "UNCHANGED", "still-here"),
		PullDoesNotContain("charlie", "old-token"),
		PullContainsAll("charlie", "API_TOKEN", "new-token", "UNCHANGED", "still-here"),
	)
}

func TestScenario_FullLifecycleMultiStage(t *testing.T) {
	RunScenario(t,
		Users("founder", "early-hire", "intern", "contractor"),
		Register("founder"),
		Add("founder", "DB_URL", "postgres://prod:5432/app"),
		Add("founder", "STRIPE_KEY", "sk_live_xxx"),
		Register("early-hire"),
		PullFails("early-hire"),
		Sync("founder"),
		PullContains("early-hire", "sk_live_xxx"),
		Add("early-hire", "REDIS_URL", "redis://cache:6379"),
		Add("early-hire", "SENTRY_DSN", "https://sentry.io/xxx"),
		PullContainsAll("founder", "REDIS_URL", "SENTRY_DSN"),
		Register("intern"),
		Sync("early-hire"),
		PullContains("intern", "postgres://prod:5432/app"),
		Add("intern", "INTERN_TEST", "test-value"),
		Register("contractor"),
		Sync("founder"),
		PullContainsAll("founder", "DB_URL", "STRIPE_KEY", "REDIS_URL", "SENTRY_DSN", "INTERN_TEST"),
		PullContainsAll("early-hire", "DB_URL", "STRIPE_KEY", "REDIS_URL", "SENTRY_DSN", "INTERN_TEST"),
		PullContainsAll("intern", "DB_URL", "STRIPE_KEY", "REDIS_URL", "SENTRY_DSN", "INTERN_TEST"),
		PullContainsAll("contractor", "DB_URL", "STRIPE_KEY", "REDIS_URL", "SENTRY_DSN", "INTERN_TEST"),
	)
}
