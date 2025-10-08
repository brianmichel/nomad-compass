# Nomad Compass

Nomad Compass is a GitOps reconciler for HashiCorp Nomad. It runs as a single container that hosts a tiny onboarding UI, stores encrypted repository credentials, and continuously syncs Nomad job specifications committed to Git.

## Features

- **Single container** – Vue-powered onboarding UI and Go backend served from the same binary.
- **Secure credential storage** – HTTPS tokens and SSH keys encrypted with a symmetric key supplied via configuration.
- **SQLite persistence** – Lightweight, zero-dependency database managed automatically.
- **Git polling** – Uses `go-git` to clone, fetch, and track `.nomad/*.nomad.hcl` job files.
- **Nomad integration** – Parses HCL jobspecs and registers them via the Nomad API with commit metadata attached.
- **Extensive metadata** – Jobs are tagged with repository URL, commit SHA, author, and commit title for traceability.
- **Well tested** – Core encryption, storage, reconciliation, and Git plumbing covered by unit tests.

## Getting Started

### Tooling with `mise`

This project ships with a `.mise.toml` that pins Go and Node versions. After installing [`mise`](https://mise.jdx.dev/):

```bash
mise install
mise shell
```

### Configuration

Nomad Compass is configured via environment variables:

| Variable | Description | Default |
| --- | --- | --- |
| `COMPASS_HTTP_ADDR` | HTTP listener address | `:8080` |
| `COMPASS_DATABASE_PATH` | Path to SQLite database | `data/nomad-compass.sqlite` |
| `COMPASS_NOMAD_ADDR` | Nomad API address | `http://127.0.0.1:4646` |
| `COMPASS_NOMAD_TOKEN` | Nomad ACL token | _empty_ |
| `COMPASS_NOMAD_REGION` | Nomad region override | _empty_ |
| `COMPASS_NOMAD_NAMESPACE` | Nomad namespace override | _empty_ |
| `COMPASS_REPO_BASE_DIR` | Directory for cloned repositories | `data/repos` |
| `COMPASS_REPO_POLL_SECONDS` | Polling cadence (seconds) | `30` |
| `COMPASS_CREDENTIAL_KEY` | 32-byte encryption key encoded as 64 hex chars | _required_ |

> ⚠️ The encryption key is mandatory. Generate one with `openssl rand -hex 32`.

### Running locally

1. Install backend dependencies and prepare the database:

    ```bash
    go test ./...
    ```

2. Install frontend dependencies and launch the dev server (optional live reload):

    ```bash
    cd frontend
    npm install
    npm run dev
    ```

   The Vite proxy forwards `/api` requests to the Go backend on port 8080.

3. Build the production bundle and run the Go binary:

    ```bash
    npm run build        # from frontend/
    cd ..
    go run ./cmd/nomad-compass
    ```

### Docker image

Build the container:

```bash
docker build -t nomad-compass .
```

Run it:

```bash
docker run \
  -e COMPASS_CREDENTIAL_KEY=$(openssl rand -hex 32) \
  -e COMPASS_NOMAD_ADDR=http://nomad.service.consul:4646 \
  -p 8080:8080 \
  nomad-compass
```

Mount `/data` or change `COMPASS_DATABASE_PATH`/`COMPASS_REPO_BASE_DIR` if you prefer persistent volumes.

### Repository onboarding workflow

1. Create credentials in the UI (HTTPS token or SSH key). Values are encrypted before hitting disk.
2. Onboard a repository by providing display name, Git URL, branch, and optional credential.
3. Nomad Compass clones the repo and watches `.nomad/job.nomad.hcl` plus any additional `.nomad/*.nomad.hcl` files.
4. When new commits land, Compass registers each job with metadata:

   - `nomad-compass/repo-url`
   - `nomad-compass/repo-name`
   - `nomad-compass/job-file`
   - `nomad-compass/commit`
   - `nomad-compass/commit-author`
   - `nomad-compass/commit-title`

Trigger an immediate reconcile via the UI or `POST /api/repos/{id}/reconcile`.

### Testing

Run the Go test suite:

```bash
go test ./...
```

Vue component tests are not included yet. The backend carries the bulk of logic and has targeted unit coverage.

## Project Layout

```
cmd/nomad-compass     # Application entrypoint
frontend/             # Vue + Vite UI
internal/auth         # Credential encryption helpers
internal/config       # Environment-driven configuration
internal/nomadclient  # Thin Nomad API wrapper
internal/reconcile    # Reconciliation loop
internal/repo         # Git sync and job discovery
internal/server       # HTTP API and SPA hosting
internal/storage      # SQLite persistence layer
internal/web          # Embedded frontend assets
```

## Future Enhancements

- Background webhook receiver to replace polling when SCM supports it.
- Audit log of reconciliation events.
- Pluggable secret backends (Vault, AWS Secrets Manager, etc.).
- Automated e2e pipeline for Docker image smoke testing.
