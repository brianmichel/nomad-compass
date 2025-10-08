package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/brianmichel/nomad-compass/internal/reconcile"
	"github.com/brianmichel/nomad-compass/internal/storage"
	"github.com/brianmichel/nomad-compass/internal/web"
)

// Server exposes HTTP handlers for UI and API requests.
type Server struct {
	repos      *storage.RepoStore
	creds      *storage.CredentialStore
	reconciler *reconcile.Manager
	logger     *slog.Logger
}

// New constructs a Server.
func New(repos *storage.RepoStore, creds *storage.CredentialStore, reconciler *reconcile.Manager, logger *slog.Logger) *Server {
	return &Server{repos: repos, creds: creds, reconciler: reconciler, logger: logger}
}

// Handler builds the router hierarchy.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Route("/api", func(api chi.Router) {
		api.Get("/repos", s.handleListRepos)
		api.Post("/repos", s.handleCreateRepo)
		api.Post("/repos/{id}/reconcile", s.handleTriggerRepo)

		api.Get("/credentials", s.handleListCredentials)
		api.Post("/credentials", s.handleCreateCredential)
	})

	distFS, err := fs.Sub(web.FS(), "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServerFS(distFS)
	r.Handle("/*", spaHandler(distFS, fileServer))

	return r
}

func (s *Server) handleListRepos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	repos, err := s.repos.List(ctx)
	if err != nil {
		respondErr(w, err)
		return
	}
	respondJSON(w, repos)
}

func (s *Server) handleCreateRepo(w http.ResponseWriter, r *http.Request) {
	var req createRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondStatus(w, http.StatusBadRequest, err)
		return
	}

	repo, err := s.repos.Create(r.Context(), storage.RepositoryInput{
		Name:    req.Name,
		RepoURL: req.RepoURL,
		Branch:  req.Branch,
		CredentialID: sql.NullInt64{
			Int64: req.CredentialID,
			Valid: req.CredentialID > 0,
		},
	})
	if err != nil {
		respondErr(w, err)
		return
	}

	go func(repoID int64) {
		if err := s.reconciler.ReconcileRepo(context.Background(), repoID); err != nil {
			s.logger.Error("initial reconcile failed", "repo_id", repoID, "error", err)
		}
	}(repo.ID)

	respondJSON(w, repo)
}

func (s *Server) handleTriggerRepo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondStatus(w, http.StatusBadRequest, err)
		return
	}
	if err := s.reconciler.ReconcileRepo(r.Context(), id); err != nil {
		respondErr(w, err)
		return
	}
	respondStatus(w, http.StatusAccepted, nil)
}

func (s *Server) handleListCredentials(w http.ResponseWriter, r *http.Request) {
	creds, err := s.creds.List(r.Context())
	if err != nil {
		respondErr(w, err)
		return
	}
	respondJSON(w, creds)
}

func (s *Server) handleCreateCredential(w http.ResponseWriter, r *http.Request) {
	var req createCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondStatus(w, http.StatusBadRequest, err)
		return
	}

	payload := storage.CredentialPayload{
		Token:      req.Token,
		Username:   req.Username,
		PrivateKey: req.PrivateKey,
		Passphrase: req.Passphrase,
	}

	cred, err := s.creds.Create(r.Context(), req.Name, storage.CredentialType(req.Type), payload)
	if err != nil {
		respondErr(w, err)
		return
	}
	respondJSON(w, cred)
}

type createRepoRequest struct {
	Name         string `json:"name"`
	RepoURL      string `json:"repo_url"`
	Branch       string `json:"branch"`
	CredentialID int64  `json:"credential_id"`
}

type createCredentialRequest struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Token      string `json:"token"`
	Username   string `json:"username"`
	PrivateKey string `json:"private_key"`
	Passphrase string `json:"passphrase"`
}

func respondJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		respondStatus(w, http.StatusInternalServerError, err)
	}
}

func respondStatus(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err == nil {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
		return
	}
	payload := map[string]string{"error": err.Error()}
	_ = json.NewEncoder(w).Encode(payload)
}

func respondErr(w http.ResponseWriter, err error) {
	respondStatus(w, http.StatusInternalServerError, err)
}

func spaHandler(assetFS fs.FS, fsHandler http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{ResponseWriter: w, status: 200}
		fsHandler.ServeHTTP(rw, r)
		if rw.status == http.StatusNotFound {
			index, err := assetFS.Open("index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			defer index.Close()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = io.Copy(w, index)
		}
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
