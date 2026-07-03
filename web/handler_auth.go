package web

import (
	"net/http"

	"github.com/yashikota/enbu/auth"
	"github.com/yashikota/enbu/config"
	gh "github.com/yashikota/enbu/provider/github"
)

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"authenticated": false,
	}

	token, err := auth.LoadToken()
	if err != nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	resp["authenticated"] = true
	resp["username"] = token.Username

	if cfg, err := config.LoadRepo(); err == nil {
		resp["repo"] = map[string]string{
			"owner": cfg.Owner,
			"name":  cfg.Repo,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	s.state = generateToken()
	redirectURL := auth.AuthorizeURL(s.clientID, s.state)
	writeJSON(w, http.StatusOK, map[string]string{
		"redirect_url": redirectURL,
	})
}

func (s *Server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if state == "" || state != s.state {
		writeError(w, http.StatusBadRequest, "INVALID_STATE", "invalid OAuth state")
		return
	}
	s.state = ""

	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "MISSING_CODE", "missing authorization code")
		return
	}

	tokenResp, err := auth.ExchangeCode(r.Context(), s.clientID, ClientSecret, code)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "TOKEN_EXCHANGE_FAILED", err.Error())
		return
	}

	client := gh.NewClient(tokenResp.AccessToken)
	user, err := client.GetUser(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "USER_FETCH_FAILED", err.Error())
		return
	}

	if err := auth.SaveToken(&auth.StoredToken{
		AccessToken: tokenResp.AccessToken,
		Username:    user.Login,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "TOKEN_SAVE_FAILED", err.Error())
		return
	}

	http.Redirect(w, r, "/?authenticated=true", http.StatusTemporaryRedirect)
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if err := auth.DeleteToken(); err != nil {
		writeError(w, http.StatusInternalServerError, "LOGOUT_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}
