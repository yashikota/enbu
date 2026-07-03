package web

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	env := r.URL.Query().Get("env")

	secrets, err := s.app.ListSecrets(r.Context(), env)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_SECRETS_ERROR", err.Error())
		return
	}

	items := make([]map[string]string, 0, len(secrets))
	for k, v := range secrets {
		items = append(items, map[string]string{"key": k, "value": v})
	}

	currentEnv, _ := s.app.CurrentEnvironment()
	if env == "" {
		env = currentEnv
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"environment": env,
		"secrets":     items,
	})
}

func (s *Server) handleAddSecret(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
		Env   string `json:"env"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}

	if err := s.app.AddSecret(r.Context(), req.Env, req.Key, req.Value); err != nil {
		writeError(w, http.StatusBadRequest, "ADD_SECRET_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"key": req.Key})
}

func (s *Server) handleEditSecret(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	var req struct {
		Value string `json:"value"`
		Env   string `json:"env"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}

	if err := s.app.EditSecret(r.Context(), req.Env, key, req.Value); err != nil {
		writeError(w, http.StatusBadRequest, "EDIT_SECRET_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"key": key})
}

func (s *Server) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	env := r.URL.Query().Get("env")

	if err := s.app.DeleteSecret(r.Context(), env, key); err != nil {
		writeError(w, http.StatusBadRequest, "DELETE_SECRET_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"deleted": key})
}

func (s *Server) handlePullSecrets(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Env string `json:"env"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}

	if err := s.app.PullSecretsToFile(r.Context(), req.Env); err != nil {
		writeError(w, http.StatusInternalServerError, "PULL_SECRETS_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "pulled"})
}

func (s *Server) handleSyncSecrets(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Env string `json:"env"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}

	if err := s.app.SyncSecrets(r.Context(), req.Env); err != nil {
		writeError(w, http.StatusInternalServerError, "SYNC_SECRETS_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "synced"})
}
