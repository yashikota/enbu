package desktop

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/yashikota/enbu/app"
	"github.com/yashikota/enbu/auth"
	"github.com/yashikota/enbu/config"
	gitprovider "github.com/yashikota/enbu/provider/git"
	gh "github.com/yashikota/enbu/provider/github"
)

type fakeRepositoryPlatform struct {
	owners       []gh.RepositoryOwner
	organization string
}

func (f *fakeRepositoryPlatform) ListRepositoryOwners(context.Context) ([]gh.RepositoryOwner, error) {
	return f.owners, nil
}

func (f *fakeRepositoryPlatform) CreateRepository(
	_ context.Context,
	organization string,
	name string,
	_ bool,
) (*gh.CreateRepoResult, error) {
	f.organization = organization
	owner := organization
	if owner == "" {
		owner = "octo"
	}
	return &gh.CreateRepoResult{
		Owner:    owner,
		Name:     name,
		SSHURL:   "git@github.com:" + owner + "/" + name + ".git",
		HTTPSURL: "https://github.com/" + owner + "/" + name + ".git",
	}, nil
}

type fakeServiceGit struct {
	hasRemote bool
	remote    string
}

func (f *fakeServiceGit) Inspect(_ context.Context, path string) (gitprovider.Repository, error) {
	return gitprovider.Repository{
		Root:      path,
		OriginURL: f.remote,
		HasGit:    true,
		HasRemote: f.hasRemote,
	}, nil
}

func (*fakeServiceGit) Init(context.Context, string) error { return nil }

func (f *fakeServiceGit) AddRemote(_ context.Context, _, _, url string) error {
	f.hasRemote = true
	f.remote = url
	return nil
}

func (*fakeServiceGit) CommitFiles(context.Context, string, []string, string) error { return nil }

func TestValidateRepoPath(t *testing.T) {
	repoDir := newGitRepo(t)

	repo, err := ValidateRepoPath(repoDir)
	if err != nil {
		t.Fatalf("ValidateRepoPath: %v", err)
	}
	if repo.Path != repoDir {
		t.Fatalf("Path = %q, want %q", repo.Path, repoDir)
	}
	if repo.Owner != "octo" || repo.Repo != "hello" {
		t.Fatalf("repo = %s/%s, want octo/hello", repo.Owner, repo.Repo)
	}
}

func TestValidateRepoPathRejectsMissingPath(t *testing.T) {
	if _, err := ValidateRepoPath(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("ValidateRepoPath succeeded for missing path")
	}
}

func TestGitInitInitializesSelectedDirectory(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	dir := t.TempDir()
	s := NewService(app.New(), "client")
	s.ctx = context.Background()

	repo, err := s.GitInit(dir)
	if err != nil {
		t.Fatalf("GitInit: %v", err)
	}
	if !repo.HasGit {
		t.Fatal("GitInit returned HasGit=false")
	}
	if repo.HasRemote {
		t.Fatal("GitInit returned HasRemote=true without origin")
	}
	if repo.Path != runGit(t, dir, "rev-parse", "--show-toplevel") {
		t.Fatalf("repo path = %q, want initialized directory", repo.Path)
	}
}

func TestListRepositoryOwners(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("GITHUB_ACTOR", "octo")
	want := []gh.RepositoryOwner{
		{Login: "octo"},
		{Login: "octo-org", Organization: true},
	}
	platform := &fakeRepositoryPlatform{owners: want}
	s := NewService(app.New(), "client")
	s.github = func(token string) repositoryPlatform {
		if token != "test-token" {
			t.Fatalf("token = %q", token)
		}
		return platform
	}

	owners, err := s.ListRepositoryOwners()
	if err != nil {
		t.Fatalf("ListRepositoryOwners: %v", err)
	}
	if !reflect.DeepEqual(owners, want) {
		t.Fatalf("owners = %#v, want %#v", owners, want)
	}
}

func TestGitCreateRemoteSelectsPersonalOrOrganizationOwner(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("GITHUB_ACTOR", "octo")

	tests := []struct {
		name             string
		owner            string
		wantOrganization string
	}{
		{name: "personal account", owner: "octo", wantOrganization: ""},
		{name: "organization", owner: "octo-org", wantOrganization: "octo-org"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitClient := &fakeServiceGit{}
			platform := &fakeRepositoryPlatform{}
			a := app.New()
			a.Git = gitClient
			s := NewService(a, "client")
			s.github = func(string) repositoryPlatform { return platform }

			repo, err := s.GitCreateRemote(t.TempDir(), tt.owner, "example", true)
			if err != nil {
				t.Fatalf("GitCreateRemote: %v", err)
			}
			if platform.organization != tt.wantOrganization {
				t.Fatalf("organization = %q, want %q", platform.organization, tt.wantOrganization)
			}
			if repo.Owner != tt.owner || !repo.HasRemote {
				t.Fatalf("repo = %#v", repo)
			}
		})
	}
}

func TestStartDeviceLoginDoesNotExposeDeviceCode(t *testing.T) {
	s := NewService(app.New(), "client")
	s.SetClipboardCopier(func(text string) error {
		if text != "ABCD-1234" {
			t.Fatalf("copied text = %q, want user code", text)
		}
		return nil
	})
	s.SetBrowserOpener(func(url string) error {
		if url != "https://github.com/login/device" {
			t.Fatalf("opened url = %q", url)
		}
		return nil
	})
	s.requestDC = func(context.Context, string) (*auth.DeviceCodeResponse, error) {
		return &auth.DeviceCodeResponse{
			DeviceCode:      "secret-device-code",
			UserCode:        "ABCD-1234",
			VerificationURI: "https://github.com/login/device",
			ExpiresIn:       900,
			Interval:        5,
		}, nil
	}
	s.pollToken = func(context.Context, string, string, int) (*auth.TokenResponse, error) {
		return nil, context.Canceled
	}

	start, err := s.StartDeviceLogin()
	if err != nil {
		t.Fatalf("StartDeviceLogin: %v", err)
	}
	if start.UserCode != "ABCD-1234" {
		t.Fatalf("UserCode = %q", start.UserCode)
	}
	if start.Copied != true || start.BrowserOpened != true {
		t.Fatalf("Copied/BrowserOpened = %v/%v", start.Copied, start.BrowserOpened)
	}
	if start.SessionID == "" {
		t.Fatal("SessionID is empty")
	}
}

func TestSelectRepositoryUpdatesHistory(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	repoDir := newGitRepo(t)
	s := NewService(app.New(), "client")
	s.ctx = context.Background()

	if _, err := s.SelectRepository(repoDir); err != nil {
		t.Fatalf("SelectRepository: %v", err)
	}

	repos, err := s.ListRepositories()
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("got %d repos, want 1", len(repos))
	}
	if repos[0].Path != repoDir {
		t.Fatalf("repo path = %q, want %q", repos[0].Path, repoDir)
	}
}

func TestListRepositoriesRemovesLegacyDuplicates(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	repoDir := newGitRepo(t)
	if err := config.SaveGUI(&config.GUIConfig{
		SelectedRepo: repoDir,
		RepoHistory:  []string{repoDir, repoDir},
	}); err != nil {
		t.Fatal(err)
	}
	s := NewService(app.New(), "client")
	s.ctx = context.Background()

	repos, err := s.ListRepositories()
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("got %d repositories, want one deduplicated repository", len(repos))
	}
}

func TestRemoveRepository(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	repoDir := newGitRepo(t)
	s := NewService(app.New(), "client")
	s.ctx = context.Background()

	if _, err := s.SelectRepository(repoDir); err != nil {
		t.Fatalf("SelectRepository: %v", err)
	}

	if err := s.RemoveRepository(repoDir); err != nil {
		t.Fatalf("RemoveRepository: %v", err)
	}

	repos, err := s.ListRepositories()
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if len(repos) != 0 {
		t.Fatalf("got %d repos after remove, want 0", len(repos))
	}

	cfg, err := config.LoadGUI()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SelectedRepo != "" {
		t.Fatalf("SelectedRepo = %q, want empty after removing active repo", cfg.SelectedRepo)
	}
}

func TestRemoveRepositoryMatchesNormalizedPath(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	repoDir := newGitRepo(t)
	s := NewService(app.New(), "client")
	s.ctx = context.Background()
	if _, err := s.SelectRepository(repoDir); err != nil {
		t.Fatalf("SelectRepository: %v", err)
	}

	pathWithDot := repoDir + string(filepath.Separator) + "."
	if err := s.RemoveRepository(pathWithDot); err != nil {
		t.Fatalf("RemoveRepository: %v", err)
	}
	cfg, err := config.LoadGUI()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.RepoHistory) != 0 || cfg.SelectedRepo != "" {
		t.Fatalf("repository was not removed: %#v", cfg)
	}
	status, err := s.GetRepoStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Path != "" {
		t.Fatalf("selected repository remains: %#v", status)
	}
}

func newGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "remote", "add", "origin", "https://github.com/octo/hello.git")
	return runGit(t, dir, "rev-parse", "--show-toplevel")
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
	return string(bytes.TrimSpace(out))
}
