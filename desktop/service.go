package desktop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yashikota/enbu/app"
	"github.com/yashikota/enbu/auth"
	"github.com/yashikota/enbu/config"
	gitprovider "github.com/yashikota/enbu/provider/git"
	gh "github.com/yashikota/enbu/provider/github"
)

type DirectoryPicker func(context.Context) (string, error)
type BrowserOpener func(string) error
type ClipboardCopier func(string) error

type Service struct {
	app       *app.App
	clientID  string
	ctx       context.Context
	pickDir   DirectoryPicker
	openURL   BrowserOpener
	copyText  ClipboardCopier
	git       gitprovider.Client
	repoMu    sync.Mutex
	authMu    sync.Mutex
	repoPath  string
	sessions  map[string]*deviceSession
	requestDC func(context.Context, string) (*auth.DeviceCodeResponse, error)
	pollToken func(context.Context, string, string, int) (*auth.TokenResponse, error)
}

type deviceSession struct {
	cancel    context.CancelFunc
	expiresAt time.Time
	status    DeviceStatus
}

func NewService(a *app.App, clientID string) *Service {
	s := &Service{
		app:       a,
		clientID:  clientID,
		openURL:   auth.OpenBrowser,
		copyText:  auth.CopyToClipboard,
		git:       a.Git,
		sessions:  make(map[string]*deviceSession),
		requestDC: auth.RequestDeviceCode,
		pollToken: auth.PollForToken,
	}
	if s.git == nil {
		s.git = gitprovider.NewCLIClient()
	}
	s.loadSelectedRepo()
	return s
}

func (s *Service) Startup(ctx context.Context) {
	slog.Info("Service.Startup called")
	s.ctx = ctx
}

func (s *Service) Context() context.Context {
	return s.ctx
}

func (s *Service) SetDirectoryPicker(picker DirectoryPicker) {
	s.pickDir = picker
}

func (s *Service) SetBrowserOpener(opener BrowserOpener) {
	s.openURL = opener
}

func (s *Service) SetClipboardCopier(copier ClipboardCopier) {
	s.copyText = copier
}

type AuthStatus struct {
	Authenticated bool      `json:"authenticated"`
	Username      string    `json:"username,omitempty"`
	Repo          *RepoInfo `json:"repo,omitempty"`
}

type RepoInfo struct {
	Path        string `json:"path,omitempty"`
	Owner       string `json:"owner,omitempty"`
	Repo        string `json:"repo,omitempty"`
	Initialized bool   `json:"initialized"`
	HasGit      bool   `json:"has_git"`
	HasRemote   bool   `json:"has_remote"`
}

type DeviceStart struct {
	SessionID       string    `json:"session_id"`
	UserCode        string    `json:"user_code"`
	VerificationURI string    `json:"verification_uri"`
	ExpiresAt       time.Time `json:"expires_at"`
	Interval        int       `json:"interval"`
	Copied          bool      `json:"copied"`
	BrowserOpened   bool      `json:"browser_opened"`
}

type DeviceStatus struct {
	State    string `json:"state"`
	Message  string `json:"message,omitempty"`
	Username string `json:"username,omitempty"`
}

type Environment struct {
	Name    string `json:"name"`
	Current bool   `json:"current"`
}

type SecretsResponse struct {
	Environment string       `json:"environment"`
	Secrets     []SecretItem `json:"secrets"`
}

type SecretItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type HistoryEntry struct {
	Index     int       `json:"index"`
	Timestamp time.Time `json:"timestamp"`
	Tag       string    `json:"tag"`
}

type Recipient struct {
	Username    string `json:"username"`
	Fingerprint string `json:"fingerprint"`
	PublicKey   string `json:"public_key"`
}

func (s *Service) GetAuthStatus() (AuthStatus, error) {
	var status AuthStatus
	token, err := auth.LoadToken()
	if err != nil {
		slog.Debug("GetAuthStatus: not authenticated", "err", err)
		return status, nil
	}
	status.Authenticated = true
	status.Username = token.Username
	if repo, err := s.GetRepoStatus(); err == nil && repo.Owner != "" {
		status.Repo = &repo
	}
	slog.Info("GetAuthStatus", "authenticated", status.Authenticated, "username", status.Username, "repo", status.Repo)
	return status, nil
}

func (s *Service) StartDeviceLogin() (DeviceStart, error) {
	slog.Info("StartDeviceLogin called")
	ctx := s.context()
	resp, err := s.requestDC(ctx, s.clientID)
	if err != nil {
		return DeviceStart{}, err
	}

	sessionID := randomSessionID()
	expiresAt := time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
	loginCtx, cancel := context.WithDeadline(ctx, expiresAt)

	copied := false
	if s.copyText != nil {
		copied = s.copyText(resp.UserCode) == nil
	}
	opened := false
	if s.openURL != nil {
		opened = s.openURL(resp.VerificationURI) == nil
	}

	s.authMu.Lock()
	for _, session := range s.sessions {
		session.cancel()
	}
	s.sessions = map[string]*deviceSession{
		sessionID: {
			cancel:    cancel,
			expiresAt: expiresAt,
			status:    DeviceStatus{State: "pending"},
		},
	}
	s.authMu.Unlock()

	go s.pollDeviceLogin(loginCtx, sessionID, resp.DeviceCode, resp.Interval)

	return DeviceStart{
		SessionID:       sessionID,
		UserCode:        resp.UserCode,
		VerificationURI: resp.VerificationURI,
		ExpiresAt:       expiresAt,
		Interval:        resp.Interval,
		Copied:          copied,
		BrowserOpened:   opened,
	}, nil
}

func (s *Service) GetDeviceLoginStatus(sessionID string) (DeviceStatus, error) {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return DeviceStatus{State: "expired", Message: "login session expired"}, nil
	}
	if time.Now().After(session.expiresAt) && session.status.State == "pending" {
		session.status = DeviceStatus{State: "expired", Message: "device code expired"}
	}
	return session.status, nil
}

func (s *Service) CancelDeviceLogin(sessionID string) error {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return nil
	}
	session.cancel()
	delete(s.sessions, sessionID)
	return nil
}

func (s *Service) pollDeviceLogin(ctx context.Context, sessionID, deviceCode string, interval int) {
	token, err := s.pollToken(ctx, s.clientID, deviceCode, interval)
	if err != nil {
		state := "error"
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			state = "expired"
		} else if err.Error() == "access denied by user" {
			state = "denied"
		}
		s.setDeviceStatus(sessionID, DeviceStatus{State: state, Message: err.Error()})
		return
	}

	client := gh.NewClient(token.AccessToken)
	user, err := client.GetUser(ctx)
	if err != nil {
		s.setDeviceStatus(sessionID, DeviceStatus{State: "error", Message: err.Error()})
		return
	}

	if err := auth.SaveToken(&auth.StoredToken{AccessToken: token.AccessToken, Username: user.Login}); err != nil {
		s.setDeviceStatus(sessionID, DeviceStatus{State: "error", Message: err.Error()})
		return
	}

	s.setDeviceStatus(sessionID, DeviceStatus{State: "success", Username: user.Login})
}

func (s *Service) setDeviceStatus(sessionID string, status DeviceStatus) {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	if session, ok := s.sessions[sessionID]; ok {
		session.status = status
	}
}

func (s *Service) Logout() error {
	return auth.DeleteToken()
}

func (s *Service) BrowseRepository() (RepoInfo, error) {
	if s.pickDir == nil {
		return RepoInfo{}, fmt.Errorf("directory picker is not available")
	}
	path, err := s.pickDir(s.context())
	if err != nil {
		return RepoInfo{}, err
	}
	if path == "" {
		return RepoInfo{}, nil
	}
	return s.SelectRepository(path)
}

func (s *Service) SelectRepository(path string) (RepoInfo, error) {
	repo, err := validateRepoPath(s.context(), s.git, path)
	if err != nil {
		return RepoInfo{}, err
	}
	s.repoMu.Lock()
	cfg, err := config.LoadGUI()
	if err != nil {
		cfg = &config.GUIConfig{}
	}
	cfg.SelectedRepo = repo.Path
	cfg.RepoHistory = appendUnique(cfg.RepoHistory, repo.Path)
	saveErr := config.SaveGUI(cfg)
	if saveErr == nil {
		s.repoPath = repo.Path
	}
	s.repoMu.Unlock()
	if saveErr != nil {
		return RepoInfo{}, saveErr
	}
	return s.repoInfo(repo)
}

func (s *Service) ListRepositories() ([]RepoInfo, error) {
	cfg, err := config.LoadGUI()
	if err != nil {
		return nil, err
	}
	var repos []RepoInfo
	for _, path := range cfg.RepoHistory {
		repo, err := validateRepoPath(s.context(), s.git, path)
		if err != nil {
			continue
		}
		info, err := s.repoInfo(repo)
		if err != nil {
			continue
		}
		repos = append(repos, info)
	}
	return repos, nil
}

func (s *Service) RemoveRepository(path string) error {
	s.repoMu.Lock()
	defer s.repoMu.Unlock()
	cfg, err := config.LoadGUI()
	if err != nil {
		return err
	}
	filtered := cfg.RepoHistory[:0]
	for _, p := range cfg.RepoHistory {
		if p != path {
			filtered = append(filtered, p)
		}
	}
	cfg.RepoHistory = filtered
	if cfg.SelectedRepo == path {
		cfg.SelectedRepo = ""
		s.repoPath = ""
	}
	return config.SaveGUI(cfg)
}

// GitInit runs `git init` in the given path and re-selects it.
func (s *Service) GitInit(path string) (RepoInfo, error) {
	if err := s.git.Init(s.context(), path); err != nil {
		return RepoInfo{}, err
	}
	return s.SelectRepository(path)
}

// GitCreateRemote creates a GitHub repository and sets it as origin, then re-selects.
func (s *Service) GitCreateRemote(path, repoName string, private bool) (RepoInfo, error) {
	token, err := auth.LoadToken()
	if err != nil {
		return RepoInfo{}, fmt.Errorf("not authenticated: %w", err)
	}
	client := gh.NewClient(token.AccessToken)
	result, err := client.CreateRepository(s.context(), repoName, private)
	if err != nil {
		return RepoInfo{}, err
	}
	if err := s.git.AddRemote(s.context(), path, "origin", result.SSHURL); err != nil {
		// SSH失敗ならHTTPSにフォールバック
		if err2 := s.git.AddRemote(s.context(), path, "origin", result.HTTPSURL); err2 != nil {
			return RepoInfo{}, fmt.Errorf("git remote add: %w", err)
		}
	}
	return s.SelectRepository(path)
}

func (s *Service) ListRecipients() ([]Recipient, error) {
	return withRepoResult(s, func() ([]Recipient, error) {
		infos, err := s.app.ListRecipients(s.context())
		if err != nil {
			return nil, err
		}
		out := make([]Recipient, len(infos))
		for i, r := range infos {
			out[i] = Recipient{
				Username:    r.Username,
				Fingerprint: r.Fingerprint,
				PublicKey:   r.PublicKey,
			}
		}
		return out, nil
	})
}

func (s *Service) ReadConfig() (string, error) {
	return withRepoResult(s, func() (string, error) {
		return s.app.ReadConfig()
	})
}

func (s *Service) WriteConfig(content string) error {
	return s.withRepo(func() error {
		return s.app.WriteConfig(content)
	})
}

func appendUnique(slice []string, val string) []string {
	for _, v := range slice {
		if v == val {
			return slice
		}
	}
	return append(slice, val)
}

func (s *Service) GetRepoStatus() (RepoInfo, error) {
	s.repoMu.Lock()
	path := s.repoPath
	s.repoMu.Unlock()
	if path == "" {
		return RepoInfo{}, nil
	}
	repo, err := validateRepoPath(s.context(), s.git, path)
	if err != nil {
		return RepoInfo{}, err
	}
	return s.repoInfo(repo)
}

func (s *Service) Initialize() (map[string]any, error) {
	return withRepoResult(s, func() (map[string]any, error) {
		result, err := s.app.InitializeRepository(s.context())
		if err != nil {
			return nil, err
		}
		repoRoot, err := os.Getwd()
		if err == nil {
			if err := ensureGitignore(repoRoot); err != nil {
				slog.Warn("failed to update .gitignore", "err", err)
			}
		}
		return map[string]any{
			"public_key":  result.PublicKey,
			"username":    result.Username,
			"environment": result.Environment,
		}, nil
	})
}

func (s *Service) ListEnvironments() ([]Environment, error) {
	return withRepoResult(s, func() ([]Environment, error) {
		envs, err := s.app.ListEnvironments()
		if err != nil {
			return nil, err
		}
		items := make([]Environment, len(envs))
		for i, env := range envs {
			items[i] = Environment{Name: env.Name, Current: env.IsCurrent}
		}
		return items, nil
	})
}

func (s *Service) CreateEnvironment(name string) error {
	return s.withRepo(func() error { return s.app.CreateEnvironment(name) })
}

func (s *Service) SwitchEnvironment(name string) error {
	return s.withRepo(func() error { return s.app.SwitchEnvironment(name) })
}

func (s *Service) RenameEnvironment(name, newName string) error {
	return s.withRepo(func() error { return s.app.RenameEnvironment(name, newName) })
}

func (s *Service) DeleteEnvironment(name string) error {
	return s.withRepo(func() error { return s.app.DeleteEnvironment(name) })
}

func (s *Service) ListSecrets(env string) (SecretsResponse, error) {
	return withRepoResult(s, func() (SecretsResponse, error) {
		secrets, err := s.app.ListSecrets(s.context(), env)
		if err != nil {
			return SecretsResponse{}, err
		}
		currentEnv, _ := s.app.CurrentEnvironment()
		if env == "" {
			env = currentEnv
		}
		keys := make([]string, 0, len(secrets))
		for key := range secrets {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		items := make([]SecretItem, 0, len(keys))
		for _, key := range keys {
			items = append(items, SecretItem{Key: key, Value: secrets[key]})
		}
		return SecretsResponse{Environment: env, Secrets: items}, nil
	})
}

func (s *Service) AddSecret(env, key, value string) error {
	return s.withRepo(func() error { return s.app.AddSecret(s.context(), env, key, value) })
}

func (s *Service) EditSecret(env, key, value string) error {
	return s.withRepo(func() error { return s.app.EditSecret(s.context(), env, key, value) })
}

func (s *Service) DeleteSecret(env, key string) error {
	return s.withRepo(func() error { return s.app.DeleteSecret(s.context(), env, key) })
}

func (s *Service) PullSecrets(env string) error {
	return s.withRepo(func() error { return s.app.PullSecretsToFile(s.context(), env) })
}

func (s *Service) SyncSecrets(env string) error {
	return s.withRepo(func() error { return s.app.SyncSecrets(s.context(), env) })
}

func (s *Service) ListHistory(env string) ([]HistoryEntry, error) {
	return withRepoResult(s, func() ([]HistoryEntry, error) {
		entries, err := s.app.ListHistory(s.context(), env)
		if err != nil {
			return nil, err
		}
		out := make([]HistoryEntry, len(entries))
		for i, entry := range entries {
			out[i] = HistoryEntry{Index: entry.Index, Timestamp: entry.Timestamp, Tag: entry.Tag}
		}
		return out, nil
	})
}

func (s *Service) DiffHistory(env string, from, to int) (*app.Diff, error) {
	return withRepoResult(s, func() (*app.Diff, error) {
		return s.app.DiffHistory(s.context(), env, from, to)
	})
}

func (s *Service) RestoreHistory(env string, index int) error {
	return s.withRepo(func() error { return s.app.RestoreHistory(s.context(), env, index) })
}

func (s *Service) loadSelectedRepo() {
	cfg, err := config.LoadGUI()
	if err != nil || cfg.SelectedRepo == "" {
		return
	}
	repo, err := validateRepoPath(s.context(), s.git, cfg.SelectedRepo)
	if err != nil {
		return
	}
	s.repoPath = repo.Path
}

func (s *Service) repoInfo(repo *SelectedRepo) (RepoInfo, error) {
	info := RepoInfo{
		Path:      repo.Path,
		Owner:     repo.Owner,
		Repo:      repo.Repo,
		HasGit:    repo.HasGit,
		HasRemote: repo.HasRemote,
	}
	return withRepoPathResult(s, repo.Path, func() (RepoInfo, error) {
		if _, err := config.LoadProject(); err == nil {
			info.Initialized = true
		}
		return info, nil
	})
}

func (s *Service) withRepo(fn func() error) error {
	_, err := withRepoResult(s, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

func withRepoResult[T any](s *Service, fn func() (T, error)) (T, error) {
	s.repoMu.Lock()
	path := s.repoPath
	s.repoMu.Unlock()
	var zero T
	if path == "" {
		return zero, fmt.Errorf("select a Git repository first")
	}
	return withRepoPathResult(s, path, fn)
}

func withRepoPathResult[T any](s *Service, path string, fn func() (T, error)) (T, error) {
	s.repoMu.Lock()
	defer s.repoMu.Unlock()
	var zero T
	wd, err := os.Getwd()
	if err != nil {
		return zero, err
	}
	if err := os.Chdir(path); err != nil {
		return zero, err
	}
	defer func() { _ = os.Chdir(wd) }()
	return fn()
}

func (s *Service) context() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}

var desktopGitignoreEntries = []string{
	".env",
	".env.*",
	"!.env.example",
	".enbu.local",
}

func ensureGitignore(repoRoot string) error {
	path := filepath.Join(repoRoot, ".gitignore")
	existing := ""
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	}
	lines := strings.Split(existing, "\n")
	lineSet := make(map[string]bool)
	for _, l := range lines {
		lineSet[strings.TrimSpace(l)] = true
	}
	var toAdd []string
	for _, entry := range desktopGitignoreEntries {
		entry = strings.TrimSpace(entry)
		if entry != "" && !lineSet[entry] {
			toAdd = append(toAdd, entry)
			lineSet[entry] = true
		}
	}
	if len(toAdd) == 0 {
		return nil
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	_, err = f.WriteString("\n# enbu - exclude .env files\n" + strings.Join(toAdd, "\n") + "\n")
	return err
}

func randomSessionID() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())
	}
	return hex.EncodeToString(b[:])
}
