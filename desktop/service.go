package desktop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/yashikota/enbu/app"
	"github.com/yashikota/enbu/auth"
	"github.com/yashikota/enbu/config"
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
		sessions:  make(map[string]*deviceSession),
		requestDC: auth.RequestDeviceCode,
		pollToken: auth.PollForToken,
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
	repo, err := ValidateRepoPath(path)
	if err != nil {
		return RepoInfo{}, err
	}
	if err := config.SaveGUI(&config.GUIConfig{SelectedRepo: repo.Path}); err != nil {
		return RepoInfo{}, err
	}
	s.repoMu.Lock()
	s.repoPath = repo.Path
	s.repoMu.Unlock()
	return s.repoInfo(repo)
}

func (s *Service) GetRepoStatus() (RepoInfo, error) {
	s.repoMu.Lock()
	path := s.repoPath
	s.repoMu.Unlock()
	if path == "" {
		return RepoInfo{}, nil
	}
	repo, err := ValidateRepoPath(path)
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
	repo, err := ValidateRepoPath(cfg.SelectedRepo)
	if err != nil {
		return
	}
	s.repoPath = repo.Path
}

func (s *Service) repoInfo(repo *SelectedRepo) (RepoInfo, error) {
	info := RepoInfo{Path: repo.Path, Owner: repo.Owner, Repo: repo.Repo}
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

func randomSessionID() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())
	}
	return hex.EncodeToString(b[:])
}
