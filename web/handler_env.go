package web

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleListEnvironments(w http.ResponseWriter, r *http.Request) {
	envs, err := s.app.ListEnvironments()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_ENV_ERROR", err.Error())
		return
	}

	items := make([]map[string]any, len(envs))
	for i, e := range envs {
		items[i] = map[string]any{
			"name":    e.Name,
			"current": e.IsCurrent,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"environments": items})
}

func (s *Server) handleCreateEnvironment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}

	if err := s.app.CreateEnvironment(req.Name); err != nil {
		writeError(w, http.StatusBadRequest, "CREATE_ENV_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"name": req.Name})
}

func (s *Server) handleSwitchEnvironment(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.app.SwitchEnvironment(name); err != nil {
		writeError(w, http.StatusBadRequest, "SWITCH_ENV_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"current": name})
}

func (s *Server) handleRenameEnvironment(w http.ResponseWriter, r *http.Request) {
	oldName := r.PathValue("name")
	var req struct {
		NewName string `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}

	if err := s.app.RenameEnvironment(oldName, req.NewName); err != nil {
		writeError(w, http.StatusBadRequest, "RENAME_ENV_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": req.NewName})
}

func (s *Server) handleDeleteEnvironment(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.app.DeleteEnvironment(name); err != nil {
		writeError(w, http.StatusBadRequest, "DELETE_ENV_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": name})
}
