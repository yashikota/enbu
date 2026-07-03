package web

import (
	"net/http"
	"strconv"
)

func (s *Server) handleListHistory(w http.ResponseWriter, r *http.Request) {
	env := r.URL.Query().Get("env")

	entries, err := s.app.ListHistory(r.Context(), env)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_HISTORY_ERROR", err.Error())
		return
	}

	items := make([]map[string]any, len(entries))
	for i, e := range entries {
		items[i] = map[string]any{
			"index":     e.Index,
			"timestamp": e.Timestamp,
			"tag":       e.Tag,
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"entries": items})
}

func (s *Server) handleDiffHistory(w http.ResponseWriter, r *http.Request) {
	env := r.URL.Query().Get("env")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	from, err := strconv.Atoi(fromStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_FROM", "invalid 'from' parameter")
		return
	}
	to, err := strconv.Atoi(toStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_TO", "invalid 'to' parameter")
		return
	}

	diff, err := s.app.DiffHistory(r.Context(), env, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DIFF_HISTORY_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"added":    diff.Added,
		"removed":  diff.Removed,
		"modified": diff.Modified,
	})
}

func (s *Server) handleRestoreHistory(w http.ResponseWriter, r *http.Request) {
	env := r.URL.Query().Get("env")
	indexStr := r.PathValue("index")

	idx, err := strconv.Atoi(indexStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INDEX", "invalid index")
		return
	}

	if err := s.app.RestoreHistory(r.Context(), env, idx); err != nil {
		writeError(w, http.StatusInternalServerError, "RESTORE_HISTORY_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "restored"})
}
