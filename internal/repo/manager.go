package repo

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"golang.org/x/crypto/ssh"

	"github.com/brianmichel/nomad-compass/internal/storage"
)

// Manager coordinates cloning, updating, and inspecting git repositories.
type Manager struct {
	baseDir string
}

// Snapshot represents the state of a repository after syncing.
type Snapshot struct {
	RepoPath     string
	CommitHash   string
	CommitAuthor string
	CommitTitle  string
	JobFiles     []JobFile
}

// JobFile captures a job file discovered within the repo.
type JobFile struct {
	Path     string
	FullPath string
	Content  []byte
}

// NewManager constructs a repository manager with a base directory.
func NewManager(baseDir string) *Manager {
	return &Manager{baseDir: baseDir}
}

// RemoveRepo deletes the working directory for a repository if it exists.
func (m *Manager) RemoveRepo(repoID int64) error {
	repoPath := filepath.Join(m.baseDir, fmt.Sprintf("repo-%d", repoID))
	if _, err := os.Stat(repoPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	return os.RemoveAll(repoPath)
}

// Sync fetches the latest state for repo from remote and returns a snapshot.
func (m *Manager) Sync(ctx context.Context, repo storage.Repository, credential *storage.Credential, payload *storage.CredentialPayload) (*Snapshot, error) {
	if err := os.MkdirAll(m.baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create base dir: %w", err)
	}
	repoPath := filepath.Join(m.baseDir, fmt.Sprintf("repo-%d", repo.ID))

	authMethod, err := authMethodForCredential(credential, payload)
	if err != nil {
		return nil, err
	}

	refName := plumbing.NewBranchReferenceName(repo.Branch)

	gitRepo, err := gogit.PlainOpen(repoPath)
	if errors.Is(err, gogit.ErrRepositoryNotExists) {
		gitRepo, err = gogit.PlainCloneContext(ctx, repoPath, false, &gogit.CloneOptions{
			URL:           repo.RepoURL,
			ReferenceName: refName,
			SingleBranch:  true,
			Depth:         1,
			Auth:          authMethod,
		})
		if err != nil {
			os.RemoveAll(repoPath)
			return nil, fmt.Errorf("clone repo: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	} else {
		worktree, wErr := gitRepo.Worktree()
		if wErr != nil {
			return nil, fmt.Errorf("worktree: %w", wErr)
		}
		if err := worktree.PullContext(ctx, &gogit.PullOptions{
			RemoteName:    "origin",
			ReferenceName: refName,
			SingleBranch:  true,
			Force:         true,
			Auth:          authMethod,
		}); err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
			return nil, fmt.Errorf("pull repo: %w", err)
		}
		if err := worktree.Checkout(&gogit.CheckoutOptions{Branch: refName, Force: true}); err != nil {
			return nil, fmt.Errorf("checkout branch: %w", err)
		}
	}

	hash, author, title, err := headMetadata(gitRepo)
	if err != nil {
		return nil, err
	}

	jobPath := repo.JobPath
	if jobPath == "" {
		jobPath = ".nomad"
	}
	jobFiles, err := discoverJobFiles(repoPath, jobPath)
	if err != nil {
		return nil, err
	}
	if len(jobFiles) == 0 {
		searchRoot := jobPath
		if !filepath.IsAbs(searchRoot) {
			searchRoot = filepath.Join(repoPath, jobPath)
		}
		return nil, fmt.Errorf("no job files found in %s", searchRoot)
	}

	return &Snapshot{
		RepoPath:     repoPath,
		CommitHash:   hash,
		CommitAuthor: author,
		CommitTitle:  title,
		JobFiles:     jobFiles,
	}, nil
}

func headMetadata(gitRepo *gogit.Repository) (hash string, author string, title string, err error) {
	ref, err := gitRepo.Head()
	if err != nil {
		return "", "", "", fmt.Errorf("head: %w", err)
	}
	commit, err := gitRepo.CommitObject(ref.Hash())
	if err != nil {
		return "", "", "", fmt.Errorf("commit object: %w", err)
	}
	titleLine := commit.Message
	if idx := strings.Index(commit.Message, "\n"); idx > 0 {
		titleLine = commit.Message[:idx]
	}
	return ref.Hash().String(), fmt.Sprintf("%s <%s>", commit.Author.Name, commit.Author.Email), titleLine, nil
}

func discoverJobFiles(repoPath string, jobPath string) ([]JobFile, error) {
	searchRoot := jobPath
	if searchRoot == "" {
		searchRoot = ".nomad"
	}
	if !filepath.IsAbs(searchRoot) {
		searchRoot = filepath.Join(repoPath, searchRoot)
	}

	info, err := os.Stat(searchRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var paths []string
	if info.IsDir() {
		err = filepath.WalkDir(searchRoot, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				if errors.Is(walkErr, fs.ErrNotExist) {
					return nil
				}
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if !hasNomadExtension(d.Name()) {
				return nil
			}
			paths = append(paths, path)
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		if hasNomadExtension(info.Name()) {
			paths = append(paths, searchRoot)
		}
	}

	var files []JobFile
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(repoPath, path)
		if err != nil {
			rel = path
		}
		files = append(files, JobFile{Path: rel, FullPath: path, Content: data})
	}
	return files, nil
}

func hasNomadExtension(name string) bool {
	return strings.HasSuffix(name, ".nomad") || strings.HasSuffix(name, ".nomad.hcl")
}

func authMethodForCredential(cred *storage.Credential, payload *storage.CredentialPayload) (transport.AuthMethod, error) {
	if cred == nil {
		return nil, nil
	}
	switch cred.Type {
	case storage.CredentialTypeHTTPToken:
		username := payload.Username
		if username == "" {
			username = "token"
		}
		return &githttp.BasicAuth{Username: username, Password: payload.Token}, nil
	case storage.CredentialTypeSSHKey:
		signer, err := parseSSHKey(payload.PrivateKey, payload.Passphrase)
		if err != nil {
			return nil, err
		}
		return &gitssh.PublicKeys{User: "git", Signer: signer}, nil
	default:
		return nil, fmt.Errorf("unsupported credential type: %s", cred.Type)
	}
}

func parseSSHKey(key string, passphrase string) (ssh.Signer, error) {
	if passphrase != "" {
		return ssh.ParsePrivateKeyWithPassphrase([]byte(key), []byte(passphrase))
	}
	return ssh.ParsePrivateKey([]byte(key))
}
