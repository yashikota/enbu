//go:build scenario

package test

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

	"github.com/yashikota/enbu/pkg/age"
	enbucli "github.com/yashikota/enbu/pkg/cli"
	"github.com/yashikota/enbu/pkg/keystore"
	"github.com/yashikota/enbu/pkg/oci"
	"github.com/yashikota/enbu/pkg/provider/github"
)

type testUser struct {
	svc     *enbucli.Service
	keyPair *age.KeyPair
	name    string
}

type ScenarioState struct {
	ctx         context.Context
	owner       string
	repo        string
	registryRef string
	users       map[string]*testUser
}

type Step struct {
	name string
	run  func(t *testing.T, s *ScenarioState)
}

func StepFunc(name string, fn func(t *testing.T, s *ScenarioState)) Step {
	return Step{name: name, run: fn}
}

func RunScenario(t *testing.T, steps ...Step) {
	t.Helper()

	owner, repo := uniqueRepo(t)
	state := &ScenarioState{
		ctx:         context.Background(),
		owner:       owner,
		repo:        repo,
		registryRef: fmt.Sprintf("localhost:5000/%s/%s-enbu", owner, repo),
		users:       make(map[string]*testUser),
	}

	for _, step := range steps {
		if ok := t.Run(step.name, func(t *testing.T) {
			step.run(t, state)
		}); !ok {
			t.FailNow()
		}
	}
}

func Users(names ...string) Step {
	return StepFunc(fmt.Sprintf("users %s", strings.Join(names, ", ")), func(t *testing.T, s *ScenarioState) {
		for _, name := range names {
			if _, ok := s.users[name]; ok {
				t.Fatalf("duplicate user %q", name)
			}
			s.users[name] = setupTestUser(t, s.owner, s.repo, name)
		}
	})
}

func Register(user string) Step {
	return StepFunc(fmt.Sprintf("%s registers recipient", user), func(t *testing.T, s *ScenarioState) {
		registerRecipient(t, s.ctx, s.registryRef, s.user(t, user))
	})
}

func Add(user, key, value string) Step {
	return StepFunc(fmt.Sprintf("%s adds %s", user, key), func(t *testing.T, s *ScenarioState) {
		addSecret(t, s.ctx, s.user(t, user), key, value)
	})
}

func AddFails(user, key, value string) Step {
	return StepFunc(fmt.Sprintf("%s add %s fails", user, key), func(t *testing.T, s *ScenarioState) {
		if err := addSecretExpectFail(t, s.ctx, s.user(t, user), key, value); err == nil {
			t.Fatalf("expected %s add %s to fail", user, key)
		}
	})
}

func Sync(user string) Step {
	return StepFunc(fmt.Sprintf("%s syncs", user), func(t *testing.T, s *ScenarioState) {
		syncSecrets(t, s.ctx, s.user(t, user))
	})
}

func PullFails(user string) Step {
	return StepFunc(fmt.Sprintf("%s pull fails", user), func(t *testing.T, s *ScenarioState) {
		if err := pullExpectFail(t, s.ctx, s.user(t, user)); err == nil {
			t.Fatalf("expected %s pull to fail", user)
		}
	})
}

func PullContains(user, expected string) Step {
	return PullContainsAll(user, expected)
}

func PullContainsAll(user string, expected ...string) Step {
	return StepFunc(fmt.Sprintf("%s pull contains %s", user, strings.Join(expected, ", ")), func(t *testing.T, s *ScenarioState) {
		output := pullStdout(t, s.ctx, s.user(t, user))
		for _, want := range expected {
			if !strings.Contains(output, want) {
				t.Fatalf("%s pull missing %q: %s", user, want, output)
			}
		}
	})
}

func PullDoesNotContain(user string, unexpected ...string) Step {
	return StepFunc(fmt.Sprintf("%s pull excludes %s", user, strings.Join(unexpected, ", ")), func(t *testing.T, s *ScenarioState) {
		output := pullStdout(t, s.ctx, s.user(t, user))
		for _, notWant := range unexpected {
			if strings.Contains(output, notWant) {
				t.Fatalf("%s pull unexpectedly contained %q: %s", user, notWant, output)
			}
		}
	})
}

func (s *ScenarioState) user(t *testing.T, name string) *testUser {
	t.Helper()
	user, ok := s.users[name]
	if !ok {
		t.Fatalf("unknown scenario user %q", name)
	}
	return user
}

func uniqueRepo(t *testing.T) (owner, repo string) {
	t.Helper()
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("generating random bytes: %v", err)
	}
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
	if err := ks.Store("enbu", repoKey, []byte(kp.Identity.String())); err != nil {
		t.Fatalf("storing key for %s: %v", username, err)
	}

	svc := &enbucli.Service{
		RegistryHost:  "localhost:5000",
		Registry:      &defaultRegistry{},
		TokenProvider: &mockTokenProvider{accessToken: "", username: username},
		KeyStore:      ks,
		RepoDetector:  &mockRepoDetector{owner: owner, repo: repo},
		GitHub:        &mockGitHubClient{orgs: map[string]bool{}},
	}

	return &testUser{svc: svc, keyPair: kp, name: username}
}

type defaultRegistry struct{}

func (d *defaultRegistry) Push(ctx context.Context, ref string, mediaType string, data []byte, token string, opts *oci.PushOptions) error {
	return oci.Push(ctx, ref, mediaType, data, token, opts)
}

func (d *defaultRegistry) Pull(ctx context.Context, ref string, token string) ([]byte, error) {
	return oci.Pull(ctx, ref, token)
}

func (d *defaultRegistry) ListTags(ctx context.Context, ref string, token string) ([]string, error) {
	return oci.ListTags(ctx, ref, token)
}

func (d *defaultRegistry) GetDigest(ctx context.Context, ref string, token string) (string, error) {
	return oci.GetDigest(ctx, ref, token)
}

type mockTokenProvider struct {
	accessToken string
	username    string
}

func (m *mockTokenProvider) LoadToken() (string, string, error) {
	return m.accessToken, m.username, nil
}

type mockRepoDetector struct {
	owner string
	repo  string
}

func (m *mockRepoDetector) LoadRepo() (string, string, error) {
	return m.owner, m.repo, nil
}

type mockKeyStore struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func newMockKeyStore() *mockKeyStore {
	return &mockKeyStore{data: make(map[string][]byte)}
}

func (m *mockKeyStore) Store(_, key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = append([]byte(nil), value...)
	return nil
}

func (m *mockKeyStore) Load(_, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.data[key]
	if !ok {
		return nil, keystore.ErrNotFound
	}
	return append([]byte(nil), d...), nil
}

type mockGitHubClient struct {
	user *github.User
	orgs map[string]bool
}

func (m *mockGitHubClient) GetUser(_ context.Context) (*github.User, error) {
	return m.user, nil
}

func (m *mockGitHubClient) IsOrganization(_ context.Context, login string) bool {
	return m.orgs[login]
}

func repoKeystoreKey(owner, repo string) string {
	return fmt.Sprintf("%s/%s", strings.ToLower(owner), strings.ToLower(repo))
}

func registerRecipient(t *testing.T, ctx context.Context, registryRef string, user *testUser) {
	t.Helper()
	fingerprint := age.Fingerprint(user.keyPair.PublicKey)
	tag := oci.CleanTag(fmt.Sprintf("%s-%s", user.name, fingerprint))
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

	var buf bytes.Buffer
	readErr := make(chan error, 1)
	go func() {
		_, err := buf.ReadFrom(r)
		readErr <- err
	}()

	origStdout := os.Stdout
	os.Stdout = w
	defer func() {
		os.Stdout = origStdout
		_ = w.Close()
		_ = r.Close()
	}()

	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("closing stdout pipe: %v", err)
	}

	if err := <-readErr; err != nil {
		t.Fatalf("reading stdout: %v", err)
	}
	return buf.String()
}

func pullStdout(t *testing.T, ctx context.Context, user *testUser) string {
	t.Helper()
	return captureStdout(t, func() {
		if err := executeCommand(ctx, user.svc, "pull", "--stdout"); err != nil {
			t.Fatalf("%s pull: %v", user.name, err)
		}
	})
}

func pullExpectFail(t *testing.T, ctx context.Context, user *testUser) error {
	t.Helper()
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()

	origStdout := os.Stdout
	os.Stdout = devNull
	defer func() {
		os.Stdout = origStdout
	}()

	return executeCommand(ctx, user.svc, "pull", "--stdout")
}

func addSecret(t *testing.T, ctx context.Context, user *testUser, key, value string) {
	t.Helper()
	if err := executeCommand(ctx, user.svc, "add", key, value); err != nil {
		t.Fatalf("%s add %s: %v", user.name, key, err)
	}
}

func addSecretExpectFail(t *testing.T, ctx context.Context, user *testUser, key, value string) error {
	t.Helper()
	return executeCommand(ctx, user.svc, "add", key, value)
}

func syncSecrets(t *testing.T, ctx context.Context, user *testUser) {
	t.Helper()
	if err := executeCommand(ctx, user.svc, "sync"); err != nil {
		t.Fatalf("%s sync: %v", user.name, err)
	}
}

func executeCommand(ctx context.Context, svc *enbucli.Service, args ...string) error {
	cmd := enbucli.NewWithService("test", svc)
	cmd.SetArgs(args)
	cmd.SetContext(ctx)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	return cmd.Execute()
}
