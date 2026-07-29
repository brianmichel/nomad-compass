#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
NOMAD_ADDR="http://127.0.0.1:4646"
COMPASS_ADDR="http://127.0.0.1:18080"
WORK_ROOT="${COMPASS_E2E_WORK_ROOT:-$ROOT/.tmp}"
mkdir -p "$WORK_ROOT"
WORK="$(mktemp -d "$WORK_ROOT/nomad-compass-e2e.XXXXXX")"
NOMAD_PID=""
COMPASS_PID=""
KEEP_RUNNING="${COMPASS_E2E_KEEP_RUNNING:-0}"

cleanup() {
  set +e
  if [[ "$KEEP_RUNNING" == "1" ]]; then
    echo "e2e-local: preserving the local Nomad/Compass environment at $WORK" >&2
    [[ -f "$WORK/e2e.env" ]] && echo "e2e-local: source $WORK/e2e.env for inspection" >&2
    return
  fi
  if [[ -n "$COMPASS_PID" ]]; then
    kill "$COMPASS_PID" 2>/dev/null || true
    wait "$COMPASS_PID" 2>/dev/null || true
  fi
  if [[ -n "$NOMAD_PID" ]]; then
    kill "$NOMAD_PID" 2>/dev/null || true
    wait "$NOMAD_PID" 2>/dev/null || true
  fi
  rm -rf "$WORK"
}
trap cleanup EXIT INT TERM

write_inspection_env() {
  [[ "$KEEP_RUNNING" == "1" ]] || return 0
  {
    printf 'export NOMAD_ADDR=%q\n' "$NOMAD_ADDR"
    printf 'export NOMAD_TOKEN=%q\n' "$NOMAD_TOKEN"
    printf 'export COMPASS_ADDR=%q\n' "$COMPASS_ADDR"
    printf 'export NOMAD_E2E_WORK=%q\n' "$WORK"
  } >"$WORK/e2e.env"
  printf '%s\n' "$NOMAD_PID" >"$WORK/nomad.pid"
  printf '%s\n' "$COMPASS_PID" >"$WORK/compass.pid"
  chmod 600 "$WORK/e2e.env" "$WORK/nomad.pid" "$WORK/compass.pid"
}

fail() {
  echo "e2e-local: $*" >&2
  echo "--- Nomad log ---" >&2
  sed -n '1,240p' "$WORK/nomad.log" 2>/dev/null || true
  echo "--- Nomad volume status ---" >&2
  NOMAD_ADDR="$NOMAD_ADDR" NOMAD_TOKEN="${NOMAD_TOKEN-}" nomad volume status compass-e2e-data 2>/dev/null || true
  echo "--- Nomad node status ---" >&2
  NOMAD_ADDR="$NOMAD_ADDR" NOMAD_TOKEN="${NOMAD_TOKEN-}" nomad node status -self -verbose 2>/dev/null || true
  echo "--- Nomad job volume request ---" >&2
  NOMAD_ADDR="$NOMAD_ADDR" nomad job inspect -json e2e 2>/dev/null | python3 -c 'import json,sys; j=json.load(sys.stdin); print(json.dumps(j.get("TaskGroups", [{}])[0].get("Volumes", {}), indent=2))' 2>/dev/null || true
  echo "--- Nomad job status ---" >&2
  NOMAD_ADDR="$NOMAD_ADDR" nomad job status e2e 2>/dev/null || true
  echo "--- Compass log ---" >&2
  sed -n '1,240p' "$WORK/compass.log" 2>/dev/null || true
  exit 1
}

for command in nomad git curl python3 openssl docker; do
  command -v "$command" >/dev/null 2>&1 || fail "missing required command: $command"
done

if ! docker info >/dev/null 2>&1; then
  for socket in \
    "${LIMA_DOCKER_SOCKET:-}" \
    "$HOME/.lima/${LIMA_INSTANCE:-default}/sock/docker.sock" \
    "$HOME/.colima/${LIMA_INSTANCE:-default}/docker.sock" \
    "$HOME"/.lima/*/sock/docker.sock \
    "$HOME"/.colima/*/docker.sock; do
    if [[ -S "$socket" ]]; then
      export DOCKER_HOST="unix://$socket"
      docker info >/dev/null 2>&1 && break
    fi
  done
fi
if ! docker info >/dev/null 2>&1; then
  fail "Docker daemon unavailable. A running limactl VM may be plain/containerd; start a Docker-enabled Lima VM (limactl start --name=docker template://docker), then set LIMA_INSTANCE=docker or LIMA_DOCKER_SOCKET explicitly"
fi
DOCKER_ENDPOINT="${DOCKER_HOST:-$(docker context inspect --format '{{ .Endpoints.docker.Host }}')}"
docker image inspect busybox:1.36 >/dev/null 2>&1 || docker pull busybox:1.36 >/dev/null

curl -fsS "$NOMAD_ADDR/v1/status/leader" >/dev/null 2>&1 && fail "Nomad is already running on 127.0.0.1:4646"
curl -fsS "$COMPASS_ADDR/api/health" >/dev/null 2>&1 && fail "Compass is already running on 127.0.0.1:18080"

mkdir -p "$WORK/nomad-data" "$WORK/nomad-volumes" "$WORK/repo/.nomad"
cat > "$WORK/nomad.hcl" <<EOF
 datacenter = "dc1"
 data_dir   = "$WORK/nomad-data"
 bind_addr  = "127.0.0.1"

 advertise {
   http = "127.0.0.1"
   rpc  = "127.0.0.1"
   serf = "127.0.0.1"
 }

 server {
   enabled          = true
   bootstrap_expect = 1
 }

 client {
   enabled          = true
   host_volumes_dir = "$WORK/nomad-volumes"
 }

 plugin "docker" {
   config {
     endpoint = "$DOCKER_ENDPOINT"
   }
 }

 acl {
   enabled = true
 }
EOF

NOMAD_ADDR="$NOMAD_ADDR" nomad agent -config="$WORK/nomad.hcl" >"$WORK/nomad.log" 2>&1 &
NOMAD_PID=$!

for _ in $(seq 1 60); do
  if curl -fsS "$NOMAD_ADDR/v1/status/leader" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl -fsS "$NOMAD_ADDR/v1/status/leader" >/dev/null 2>&1 || fail "Nomad did not become ready"

BOOTSTRAP_JSON="$(NOMAD_ADDR="$NOMAD_ADDR" nomad acl bootstrap -json)"
export NOMAD_TOKEN="$(printf '%s' "$BOOTSTRAP_JSON" | python3 -c 'import json, sys; print(json.load(sys.stdin)["SecretID"])')"
export NOMAD_ADDR

cat > "$WORK/repo/.nomad/compass.bundle.hcl" <<'EOF'
bundle "local-e2e" {
  resource "volume" "data" {
    delete = "protect"

    name      = "compass-e2e-data"
    type      = "host"
    plugin_id = "mkdir"

    capability {
      access_mode     = "single-node-single-writer"
      attachment_mode = "file-system"
    }
  }

  resource "acl_policy" "e2e" {
    delete = "protect"

    rules {
      namespace "default" {
        capabilities = [
          "read-job",
          "submit-job",
        ]
      }
    }
  }

  resource "job" "e2e" {
    depends_on = [
      "volume.data",
      "acl_policy.e2e",
    ]

    datacenters = ["dc1"]
    type        = "service"

    group "app" {
      count = 1

      # The volume resource is validated through the Nomad volume API. The
      # task stays independent so this host-side Nomad client can schedule a
      # Docker task through the Lima daemon without crossing mount namespaces.
      task "verify" {
        driver = "docker"

        config {
          image   = "busybox:1.36"
          command = "sh"
          args    = ["-c", "echo bundle-e2e-ok && sleep 3600"]
        }

        resources {
          cpu    = 1
          memory = 32
        }
      }
    }
  }
}
EOF

cd "$WORK/repo"
git init --initial-branch=main >/dev/null
git config user.email "compass-e2e@example.com"
git config user.name "Compass E2E"
git add .
git commit -m "initial bundle" >/dev/null

export COMPASS_HTTP_ADDR="127.0.0.1:18080"
export COMPASS_DATABASE_PATH="$WORK/compass.sqlite"
export COMPASS_REPO_BASE_DIR="$WORK/clones"
export COMPASS_REPO_POLL_SECONDS=2
export COMPASS_NOMAD_ADDR="$NOMAD_ADDR"
export COMPASS_NOMAD_TOKEN="$NOMAD_TOKEN"
export COMPASS_CREDENTIAL_KEY="$(openssl rand -hex 32)"

cd "$ROOT"
go build -o "$WORK/nomad-compass" ./cmd/nomad-compass
"$WORK/nomad-compass" >"$WORK/compass.log" 2>&1 &
COMPASS_PID=$!
write_inspection_env

for _ in $(seq 1 60); do
  if curl -fsS "$COMPASS_ADDR/api/health" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl -fsS "$COMPASS_ADDR/api/health" >/dev/null 2>&1 || fail "Compass did not become ready"

RESPONSE="$(curl -fsS -X POST "$COMPASS_ADDR/api/repos" \
  -H 'Content-Type: application/json' \
  -d "$(python3 -c 'import json,sys; print(json.dumps({"name":"local-e2e","repo_url":sys.argv[1],"branch":"main","job_path":".nomad"}))' "$WORK/repo")")"
REPO_ID="$(printf '%s' "$RESPONSE" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"

for _ in $(seq 1 60); do
  if NOMAD_ADDR="$NOMAD_ADDR" nomad volume status -type=host compass-e2e-data >/dev/null 2>&1 \
    && NOMAD_ADDR="$NOMAD_ADDR" nomad acl policy info e2e >/dev/null 2>&1 \
    && NOMAD_ADDR="$NOMAD_ADDR" nomad job status e2e 2>/dev/null | grep "running" >/dev/null; then
    break
  fi
  sleep 1
done

NOMAD_ADDR="$NOMAD_ADDR" nomad volume status -type=host compass-e2e-data >/dev/null 2>&1 || fail "host volume was not created"
NOMAD_ADDR="$NOMAD_ADDR" nomad acl policy info e2e >/dev/null 2>&1 || fail "ACL policy was not created"
NOMAD_ADDR="$NOMAD_ADDR" nomad job status e2e | grep "running" >/dev/null || fail "job did not reach running state"

curl -fsS -X POST "$COMPASS_ADDR/api/repos/$REPO_ID/reconcile" >/dev/null
sleep 2
NOMAD_ADDR="$NOMAD_ADDR" nomad volume status -type=host compass-e2e-data >/dev/null || fail "volume disappeared after idempotent reconcile"
NOMAD_ADDR="$NOMAD_ADDR" nomad acl policy info e2e >/dev/null || fail "policy disappeared after idempotent reconcile"
NOMAD_ADDR="$NOMAD_ADDR" nomad job status e2e | grep "running" >/dev/null || fail "job stopped after idempotent reconcile"

printf '\nLocal bundle E2E passed.\n'
printf '  repository: %s\n' "$REPO_ID"
printf '  Nomad:      %s\n' "$NOMAD_ADDR"
printf '  Compass:    %s\n' "$COMPASS_ADDR"
printf '  logs:       %s\n' "$WORK"
if [[ "$KEEP_RUNNING" == "1" ]]; then
  printf '  inspect:    source %s/e2e.env\n' "$WORK"
  printf '  stop:       kill $(cat %s/compass.pid %s/nomad.pid)\n' "$WORK" "$WORK"
fi
