package web

import (
	"errors"
	"net/http"
	"strings"

	agecrypto "filippo.io/age"
	"github.com/yashikota/enbu/app"
	"github.com/yashikota/enbu/config"
	gh "github.com/yashikota/enbu/provider/github"
	"github.com/yashikota/enbu/utils/age"
	"github.com/yashikota/enbu/utils/keystore"
	"github.com/yashikota/enbu/utils/oci"
)

func (s *Server) handleRepoStatus(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"initialized": false,
	}

	cfg, err := config.LoadRepo()
	if err != nil {
		writeError(w, http.StatusBadRequest, "NO_REPO", "not in a git repository")
		return
	}
	resp["owner"] = cfg.Owner
	resp["repo"] = cfg.Repo

	if _, err := config.LoadProject(); err == nil {
		resp["initialized"] = true
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleInit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	accessToken, username, err := s.app.TokenProvider.LoadToken()
	if err != nil {
		writeError(w, http.StatusUnauthorized, "NOT_AUTHENTICATED", "not logged in")
		return
	}

	owner, repo, err := s.app.RepoDetector.LoadRepo()
	if err != nil {
		writeError(w, http.StatusBadRequest, "NO_REPO", "not in a git repository")
		return
	}

	registryRef := "ghcr.io/" + strings.ToLower(owner) + "/" + strings.ToLower(repo) + "-enbu"

	repoKey := app.RepoKeystoreKey(owner, repo)
	var publicKey string

	existingPriv, err := s.app.KeyStore.Load(app.KeystoreService, repoKey)
	if err == nil && len(existingPriv) > 0 {
		id, err := agecrypto.ParseX25519Identity(string(existingPriv))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "KEY_PARSE_ERROR", err.Error())
			return
		}
		publicKey = id.Recipient().String()
	} else if err != nil && !errors.Is(err, keystore.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "KEYSTORE_ERROR", err.Error())
		return
	} else {
		kp, err := age.GenerateKeyPair()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "KEY_GEN_ERROR", err.Error())
			return
		}
		publicKey = kp.PublicKey

		if err := s.app.KeyStore.Store(app.KeystoreService, repoKey, []byte(kp.Identity.String())); err != nil {
			writeError(w, http.StatusInternalServerError, "KEY_STORE_ERROR", err.Error())
			return
		}
	}

	ghClient := gh.NewClient(accessToken)
	fingerprint := age.Fingerprint(publicKey)
	tag := oci.CleanTag(username + "-" + fingerprint)
	ref := registryRef + ":" + app.RecipientTagPrefix() + tag
	pushOpts := &oci.PushOptions{
		SourceRepo: ghClient.SourceRepoURL(owner, repo),
	}
	if err := s.app.Registry.Push(ctx, ref, "application/vnd.enbu.recipient.age.v1", []byte(publicKey), accessToken, pushOpts); err != nil {
		writeError(w, http.StatusInternalServerError, "PUSH_ERROR", err.Error())
		return
	}

	projectCfg, err := config.LoadProject()
	if err != nil {
		projectCfg = config.NewProjectWithEnvironment(app.DefaultEnvironment)
		if err := config.SaveProject(projectCfg); err != nil {
			writeError(w, http.StatusInternalServerError, "CONFIG_SAVE_ERROR", err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"public_key":  publicKey,
		"username":    username,
		"environment": projectCfg.CurrentEnvironment(),
	})
}
