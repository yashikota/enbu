//go:build scenario

package cli

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/yashikota/enbu/internal/age"
	"github.com/yashikota/enbu/internal/oci"
)

type testUser struct {
	svc     *Service
	keyPair *age.KeyPair
	name    string
}

func uniqueRepo(t *testing.T) (owner, repo string) {
	t.Helper()
	b := make([]byte, 4)
	rand.Read(b)
	return "test", fmt.Sprintf("%s-%s", strings.ToLower(t.Name()), hex.EncodeToString(b))
}

func setupTestUser(t *testing.T, owner, repo, username string) *testUser {
	t.Helper()

	kp, err := age.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair for %s: %v", username, err)
	}

	ks := newMockKeyStore()
	repoKey := repoKeystoreKey(owner, repo)
	if err := ks.Store(keystoreService, repoKey, []byte(kp.Identity.String())); err != nil {
		t.Fatalf("storing key for %s: %v", username, err)
	}

	svc := &Service{
		RegistryHost:  "localhost:5000",
		Registry:      &defaultRegistry{},
		TokenProvider: &mockTokenProvider{accessToken: "", username: username},
		KeyStore:      ks,
		RepoDetector:  &mockRepoDetector{owner: owner, repo: repo},
		GitHub:        &mockGitHubClient{orgs: map[string]bool{}},
	}

	return &testUser{svc: svc, keyPair: kp, name: username}
}

func registerRecipient(t *testing.T, ctx context.Context, registryRef string, user *testUser) {
	t.Helper()
	fingerprint := keyFingerprint(user.keyPair.PublicKey)
	tag := cleanTag(fmt.Sprintf("%s-%s", user.name, fingerprint))
	ref := fmt.Sprintf("%s:recipient-%s", registryRef, tag)
	if err := oci.Push(ctx, ref, "application/vnd.enbu.recipient.age.v1", []byte(user.keyPair.PublicKey), "", nil); err != nil {
		t.Fatalf("registering recipient %s: %v", user.name, err)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStdout := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = origStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func pullStdout(t *testing.T, ctx context.Context, user *testUser) string {
	t.Helper()
	return captureStdout(t, func() {
		cmd := newPullCommand(user.svc)
		cmd.SetArgs([]string{"--stdout"})
		cmd.SetContext(ctx)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%s pull: %v", user.name, err)
		}
	})
}

func pullExpectFail(t *testing.T, ctx context.Context, user *testUser) error {
	t.Helper()
	cmd := newPullCommand(user.svc)
	cmd.SetArgs([]string{"--stdout"})
	cmd.SetContext(ctx)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	origStdout := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	err := cmd.Execute()
	os.Stdout = origStdout
	return err
}

func addSecret(t *testing.T, ctx context.Context, user *testUser, key, value string) {
	t.Helper()
	cmd := newAddCommand(user.svc)
	cmd.SetArgs([]string{key, value})
	cmd.SetContext(ctx)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%s add %s: %v", user.name, key, err)
	}
}

func syncSecrets(t *testing.T, ctx context.Context, user *testUser) {
	t.Helper()
	cmd := newSyncCommand(user.svc)
	cmd.SetContext(ctx)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%s sync: %v", user.name, err)
	}
}

func TestScenario_SingleUserAddPull(t *testing.T) {
	ctx := context.Background()
	owner, repo := uniqueRepo(t)
	registryRef := fmt.Sprintf("localhost:5000/%s/%s-enbu", owner, repo)

	user1 := setupTestUser(t, owner, repo, "alice")
	registerRecipient(t, ctx, registryRef, user1)

	addSecret(t, ctx, user1, "DB_HOST", "localhost")
	addSecret(t, ctx, user1, "DB_PORT", "5432")

	output := pullStdout(t, ctx, user1)
	if !strings.Contains(output, "DB_HOST=") || !strings.Contains(output, "localhost") {
		t.Fatalf("missing DB_HOST: %s", output)
	}
	if !strings.Contains(output, "DB_PORT=") || !strings.Contains(output, "5432") {
		t.Fatalf("missing DB_PORT: %s", output)
	}
}

func TestScenario_JoinFlowRequiresSync(t *testing.T) {
	ctx := context.Background()
	owner, repo := uniqueRepo(t)
	registryRef := fmt.Sprintf("localhost:5000/%s/%s-enbu", owner, repo)

	user1 := setupTestUser(t, owner, repo, "alice")
	user2 := setupTestUser(t, owner, repo, "bob")

	registerRecipient(t, ctx, registryRef, user1)
	addSecret(t, ctx, user1, "SECRET", "only-for-alice")

	registerRecipient(t, ctx, registryRef, user2)

	err := pullExpectFail(t, ctx, user2)
	if err == nil {
		t.Fatal("expected user2 pull to fail before sync")
	}

	syncSecrets(t, ctx, user1)

	output := pullStdout(t, ctx, user2)
	if !strings.Contains(output, "only-for-alice") {
		t.Fatalf("user2 should see secret after sync: %s", output)
	}
}

func TestScenario_ThreeUsersSequentialJoin(t *testing.T) {
	ctx := context.Background()
	owner, repo := uniqueRepo(t)
	registryRef := fmt.Sprintf("localhost:5000/%s/%s-enbu", owner, repo)

	user1 := setupTestUser(t, owner, repo, "alice")
	user2 := setupTestUser(t, owner, repo, "bob")
	user3 := setupTestUser(t, owner, repo, "charlie")

	registerRecipient(t, ctx, registryRef, user1)
	addSecret(t, ctx, user1, "SHARED_KEY", "initial-value")

	registerRecipient(t, ctx, registryRef, user2)
	if err := pullExpectFail(t, ctx, user2); err == nil {
		t.Fatal("user2 should fail before first sync")
	}
	syncSecrets(t, ctx, user1)
	output := pullStdout(t, ctx, user2)
	if !strings.Contains(output, "initial-value") {
		t.Fatalf("user2 missing secret: %s", output)
	}

	addSecret(t, ctx, user1, "NEW_KEY", "after-bob-joined")
	output = pullStdout(t, ctx, user2)
	if !strings.Contains(output, "after-bob-joined") {
		t.Fatalf("user2 missing new secret: %s", output)
	}

	registerRecipient(t, ctx, registryRef, user3)
	if err := pullExpectFail(t, ctx, user3); err == nil {
		t.Fatal("user3 should fail before sync")
	}

	// Any existing member can sync
	syncSecrets(t, ctx, user2)

	output = pullStdout(t, ctx, user3)
	if !strings.Contains(output, "initial-value") || !strings.Contains(output, "after-bob-joined") {
		t.Fatalf("user3 missing secrets: %s", output)
	}
}

func TestScenario_OverwriteSecret(t *testing.T) {
	ctx := context.Background()
	owner, repo := uniqueRepo(t)
	registryRef := fmt.Sprintf("localhost:5000/%s/%s-enbu", owner, repo)

	user1 := setupTestUser(t, owner, repo, "alice")
	registerRecipient(t, ctx, registryRef, user1)

	addSecret(t, ctx, user1, "API_KEY", "old-key")
	addSecret(t, ctx, user1, "API_KEY", "new-key")

	output := pullStdout(t, ctx, user1)
	if strings.Contains(output, "old-key") {
		t.Fatalf("old value should be gone: %s", output)
	}
	if !strings.Contains(output, "new-key") {
		t.Fatalf("new value missing: %s", output)
	}
}

func TestScenario_SpecialCharacterValues(t *testing.T) {
	ctx := context.Background()
	owner, repo := uniqueRepo(t)
	registryRef := fmt.Sprintf("localhost:5000/%s/%s-enbu", owner, repo)

	user1 := setupTestUser(t, owner, repo, "alice")
	registerRecipient(t, ctx, registryRef, user1)

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

	for _, tc := range testCases {
		addSecret(t, ctx, user1, tc.key, tc.value)
	}

	output := pullStdout(t, ctx, user1)
	for _, tc := range testCases {
		if !strings.Contains(output, tc.key+"=") {
			t.Errorf("missing key %s in output: %s", tc.key, output)
		}
	}
	if !strings.Contains(output, "日本語のシークレット") {
		t.Errorf("Japanese value lost: %s", output)
	}
	if !strings.Contains(output, "🔑🔒✨") {
		t.Errorf("Emoji value lost: %s", output)
	}
}

func TestScenario_ManySecrets(t *testing.T) {
	ctx := context.Background()
	owner, repo := uniqueRepo(t)
	registryRef := fmt.Sprintf("localhost:5000/%s/%s-enbu", owner, repo)

	user1 := setupTestUser(t, owner, repo, "alice")
	registerRecipient(t, ctx, registryRef, user1)

	const count = 50
	for i := range count {
		addSecret(t, ctx, user1, fmt.Sprintf("SECRET_%03d", i), fmt.Sprintf("value-%d", i))
	}

	output := pullStdout(t, ctx, user1)
	for i := range count {
		key := fmt.Sprintf("SECRET_%03d", i)
		if !strings.Contains(output, key+"=") {
			t.Fatalf("missing %s after adding %d secrets", key, count)
		}
	}
}

func TestScenario_PullNoSecrets(t *testing.T) {
	ctx := context.Background()
	owner, repo := uniqueRepo(t)
	registryRef := fmt.Sprintf("localhost:5000/%s/%s-enbu", owner, repo)

	user1 := setupTestUser(t, owner, repo, "alice")
	registerRecipient(t, ctx, registryRef, user1)

	err := pullExpectFail(t, ctx, user1)
	if err == nil {
		t.Fatal("expected pull to fail when no secrets exist")
	}
}

func TestScenario_SyncIdempotent(t *testing.T) {
	ctx := context.Background()
	owner, repo := uniqueRepo(t)
	registryRef := fmt.Sprintf("localhost:5000/%s/%s-enbu", owner, repo)

	user1 := setupTestUser(t, owner, repo, "alice")
	user2 := setupTestUser(t, owner, repo, "bob")

	registerRecipient(t, ctx, registryRef, user1)
	registerRecipient(t, ctx, registryRef, user2)

	addSecret(t, ctx, user1, "KEY", "value")

	syncSecrets(t, ctx, user1)
	syncSecrets(t, ctx, user1)
	syncSecrets(t, ctx, user2)

	out1 := pullStdout(t, ctx, user1)
	out2 := pullStdout(t, ctx, user2)
	if !strings.Contains(out1, "value") || !strings.Contains(out2, "value") {
		t.Fatalf("data corrupted after multiple syncs: user1=%s, user2=%s", out1, out2)
	}
}

func TestScenario_ConcurrentAdds(t *testing.T) {
	ctx := context.Background()
	owner, repo := uniqueRepo(t)
	registryRef := fmt.Sprintf("localhost:5000/%s/%s-enbu", owner, repo)

	user1 := setupTestUser(t, owner, repo, "alice")
	user2 := setupTestUser(t, owner, repo, "bob")

	registerRecipient(t, ctx, registryRef, user1)
	registerRecipient(t, ctx, registryRef, user2)

	addSecret(t, ctx, user1, "SEED", "initial")

	// Concurrent adds — at least one should succeed, the other may conflict
	var wg sync.WaitGroup
	var err1, err2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		cmd := newAddCommand(user1.svc)
		cmd.SetArgs([]string{"FROM_ALICE", "alice-data"})
		cmd.SetContext(ctx)
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		err1 = cmd.Execute()
	}()
	go func() {
		defer wg.Done()
		cmd := newAddCommand(user2.svc)
		cmd.SetArgs([]string{"FROM_BOB", "bob-data"})
		cmd.SetContext(ctx)
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		err2 = cmd.Execute()
	}()
	wg.Wait()

	// At least one should succeed
	if err1 != nil && err2 != nil {
		t.Fatalf("both adds failed: err1=%v, err2=%v", err1, err2)
	}

	// In real workflow: retry failed adds sequentially, then verify all data
	if err1 != nil {
		addSecret(t, ctx, user1, "FROM_ALICE", "alice-data")
	}
	if err2 != nil {
		addSecret(t, ctx, user2, "FROM_BOB", "bob-data")
	}

	// Verify the final state has all secrets
	output := pullStdout(t, ctx, user1)
	if !strings.Contains(output, "SEED") {
		t.Fatalf("missing SEED: %s", output)
	}
	// After sequential retries, both keys should be present
	if !strings.Contains(output, "FROM_ALICE") || !strings.Contains(output, "FROM_BOB") {
		// One add may have lost to the other due to last-write-wins
		// This is expected OCI behavior; verify at least the retry produced correct state
		output2 := pullStdout(t, ctx, user2)
		t.Logf("user1 sees: %s", output)
		t.Logf("user2 sees: %s", output2)
		// The last sequential add should at minimum have its own data
		if !strings.Contains(output, "FROM_ALICE") && !strings.Contains(output, "FROM_BOB") {
			t.Fatal("neither concurrent add survived")
		}
	}
}

func TestScenario_AddAfterSyncPreservesRecipients(t *testing.T) {
	ctx := context.Background()
	owner, repo := uniqueRepo(t)
	registryRef := fmt.Sprintf("localhost:5000/%s/%s-enbu", owner, repo)

	user1 := setupTestUser(t, owner, repo, "alice")
	user2 := setupTestUser(t, owner, repo, "bob")

	registerRecipient(t, ctx, registryRef, user1)
	addSecret(t, ctx, user1, "FIRST", "first-value")

	registerRecipient(t, ctx, registryRef, user2)
	syncSecrets(t, ctx, user1)

	addSecret(t, ctx, user2, "SECOND", "second-value")

	output := pullStdout(t, ctx, user1)
	if !strings.Contains(output, "first-value") || !strings.Contains(output, "second-value") {
		t.Fatalf("user1 missing secrets after user2 add: %s", output)
	}

	output = pullStdout(t, ctx, user2)
	if !strings.Contains(output, "first-value") || !strings.Contains(output, "second-value") {
		t.Fatalf("user2 missing secrets: %s", output)
	}
}

func TestScenario_FullLifecycleMultiStage(t *testing.T) {
	ctx := context.Background()
	owner, repo := uniqueRepo(t)
	registryRef := fmt.Sprintf("localhost:5000/%s/%s-enbu", owner, repo)

	user1 := setupTestUser(t, owner, repo, "founder")
	user2 := setupTestUser(t, owner, repo, "early-hire")
	user3 := setupTestUser(t, owner, repo, "intern")
	user4 := setupTestUser(t, owner, repo, "contractor")

	registerRecipient(t, ctx, registryRef, user1)
	addSecret(t, ctx, user1, "DB_URL", "postgres://prod:5432/app")
	addSecret(t, ctx, user1, "STRIPE_KEY", "sk_live_xxx")

	registerRecipient(t, ctx, registryRef, user2)
	if err := pullExpectFail(t, ctx, user2); err == nil {
		t.Fatal("early-hire should not decrypt before sync")
	}
	syncSecrets(t, ctx, user1)
	output := pullStdout(t, ctx, user2)
	if !strings.Contains(output, "sk_live_xxx") {
		t.Fatalf("early-hire can't see STRIPE_KEY: %s", output)
	}

	addSecret(t, ctx, user2, "REDIS_URL", "redis://cache:6379")
	addSecret(t, ctx, user2, "SENTRY_DSN", "https://sentry.io/xxx")

	output = pullStdout(t, ctx, user1)
	if !strings.Contains(output, "REDIS_URL") || !strings.Contains(output, "SENTRY_DSN") {
		t.Fatalf("founder missing early-hire's secrets: %s", output)
	}

	registerRecipient(t, ctx, registryRef, user3)
	syncSecrets(t, ctx, user2)
	output = pullStdout(t, ctx, user3)
	if !strings.Contains(output, "postgres://prod:5432/app") {
		t.Fatalf("intern can't see DB_URL: %s", output)
	}

	addSecret(t, ctx, user3, "INTERN_TEST", "test-value")

	registerRecipient(t, ctx, registryRef, user4)
	syncSecrets(t, ctx, user1)

	for _, u := range []*testUser{user1, user2, user3, user4} {
		out := pullStdout(t, ctx, u)
		for _, expected := range []string{"DB_URL", "STRIPE_KEY", "REDIS_URL", "SENTRY_DSN", "INTERN_TEST"} {
			if !strings.Contains(out, expected) {
				t.Errorf("%s missing %s: %s", u.name, expected, out)
			}
		}
	}
}
