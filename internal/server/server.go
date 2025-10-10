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

	"github.com/brianmichel/nomad-compass/internal/nomadclient"
	"github.com/brianmichel/nomad-compass/internal/reconcile"
	"github.com/brianmichel/nomad-compass/internal/storage"
	"github.com/brianmichel/nomad-compass/internal/web"
)

// Server exposes HTTP handlers for UI and API requests.
type Server struct {
	repos      *storage.RepoStore
	files      *storage.RepoFileStore
	creds      *storage.CredentialStore
	reconciler *reconcile.Manager
	nomad      nomadclient.Client
	logger     *slog.Logger
}

// New constructs a Server.
func New(repos *storage.RepoStore, files *storage.RepoFileStore, creds *storage.CredentialStore, reconciler *reconcile.Manager, nomad nomadclient.Client, logger *slog.Logger) *Server {
	return &Server{
		repos:      repos,
		files:      files,
		creds:      creds,
		reconciler: reconciler,
		nomad:      nomad,
		logger:     logger,
	}
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
		api.Get("/status", s.handleStatus)
		api.Get("/repos", s.handleListRepos)
		api.Post("/repos", s.handleCreateRepo)
		api.Post("/repos/{id}/reconcile", s.handleTriggerRepo)
		api.Delete("/repos/{id}", s.handleDeleteRepo)

		api.Get("/credentials", s.handleListCredentials)
		api.Post("/credentials", s.handleCreateCredential)
		api.Delete("/credentials/{id}", s.handleDeleteCredential)
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
	resp := make([]repositoryResponse, 0, len(repos))
	for _, repo := range repos {
		files, err := s.files.ListByRepo(ctx, repo.ID)
		if err != nil {
			respondErr(w, err)
			return
		}

		jobResponses := make([]repositoryJobResponse, 0, len(files))
		for _, file := range files {
			jobResp := newRepositoryJobResponse(file)
			if file.JobID.Valid && file.JobID.String != "" {
				jobResp.JobID = file.JobID.String

				status, err := s.nomad.JobStatus(ctx, file.JobID.String)
				if err != nil {
					s.logger.Warn("fetch job status failed", "repo_id", repo.ID, "repo", repo.Name, "job_id", file.JobID.String, "error", err)
					jobResp.StatusError = err.Error()
				} else if status != nil {
					if status.Exists {
						jobResp.JobName = status.Name
						jobResp.Status = status.Status
						jobResp.StatusDescription = status.StatusDescription
					} else {
						jobResp.Status = "missing"
						jobResp.StatusDescription = "Job not found in Nomad"
					}
				}
			}

			jobResponses = append(jobResponses, jobResp)
		}

		repoResp := newRepositoryResponse(repo)
		repoResp.Jobs = jobResponses
		resp = append(resp, repoResp)
	}
	respondJSON(w, resp)
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
		JobPath: req.JobPath,
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

	respondJSON(w, newRepositoryResponse(*repo))
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

func (s *Server) handleDeleteRepo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondStatus(w, http.StatusBadRequest, err)
		return
	}

	var req deleteRepoRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
			respondStatus(w, http.StatusBadRequest, err)
			return
		}
	}

	if err := s.reconciler.DeleteRepository(r.Context(), id, req.Unschedule); err != nil {
		respondErr(w, err)
		return
	}
	respondStatus(w, http.StatusOK, nil)
}

func (s *Server) handleListCredentials(w http.ResponseWriter, r *http.Request) {
	creds, err := s.creds.List(r.Context())
	if err != nil {
		respondErr(w, err)
		return
	}
	resp := make([]credentialResponse, 0, len(creds))
	for _, cred := range creds {
		resp = append(resp, newCredentialResponse(cred))
	}
	respondJSON(w, resp)
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
	respondJSON(w, newCredentialResponse(*cred))
}

func (s *Server) handleDeleteCredential(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondStatus(w, http.StatusBadRequest, err)
		return
	}

	var req deleteCredentialRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
			respondStatus(w, http.StatusBadRequest, err)
			return
		}
	}

	if err := s.reconciler.DeleteCredential(r.Context(), id, req.DeleteRepos, req.Unschedule); err != nil {
		respondErr(w, err)
		return
	}
	respondStatus(w, http.StatusOK, nil)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	err := s.nomad.Ping(r.Context())
	resp := statusResponse{NomadConnected: err == nil}
	if err != nil {
		resp.NomadMessage = err.Error()
	}
	respondJSON(w, resp)
}

type createRepoRequest struct {
	Name         string `json:"name"`
	RepoURL      string `json:"repo_url"`
	Branch       string `json:"branch"`
	JobPath      string `json:"job_path"`
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

type deleteRepoRequest struct {
	Unschedule bool `json:"unschedule"`
}

type deleteCredentialRequest struct {
	Unschedule  bool `json:"unschedule"`
	DeleteRepos bool `json:"delete_repos"`
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
