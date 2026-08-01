# Compass bundle manifest. It combines the real homelab Compass job, host
# volume, and ACL policy into one Git-managed desired state.
#
# Compass-specific fields:
#   depends_on
#   delete
#
# The remaining fields are native Nomad HCL bodies. Compass parses the
# envelope, then hands each body to the appropriate Nomad primitive adapter.

bundle "compass" {
  resource "volume" "compass_data" {
    # Compass metadata. Storage deletion must be opt-in.
    delete = "protect"

    # Adapted from homelab/nomad/volumes/compass-data.hcl.
    name      = "compass-data"
    type      = "host"
    plugin_id = "mkdir"

    capability {
      access_mode     = "single-node-single-writer"
      attachment_mode = "file-system"
    }
  }

  resource "acl_policy" "compass" {
    # ACL policy deletion should also be protected by default.
    delete = "protect"
    description = "Allows Compass to reconcile jobs from Git repositories."

    # Compass would serialize this body as the Rules field expected by
    # POST /v1/acl/policy/compass.
    rules {
      namespace "default" {
        capabilities = [
          "read-job",
          "submit-job",
        ]
      }
    }
  }

  resource "job" "compass" {
    depends_on = [
      "volume.compass_data",
      # Applying the ACL policy requires a management-capable bootstrap
      # credential. It cannot be created by the policy it grants to Compass.
      "acl_policy.compass",
    ]

    # Adapted from homelab/compass.nomad.hcl.
    datacenters = ["jobs"]

    group "compass" {
      count = 1

      volume "compass-data" {
        type            = "host"
        source          = "compass-data"
        access_mode     = "single-node-single-writer"
        attachment_mode = "file-system"
        sticky          = true
      }

      network {
        mode = "host"
        port "http" {
          host_network = "private"
          to           = 8080
        }
      }

      service {
        name     = "compass"
        provider = "nomad"
        port     = "http"
        tags = [
          "ui",
          "traefik-internal.enable=true",
          "traefik-internal.http.routers.compass.rule=Host(`compass.ttys.me`)",
        ]

        check {
          name     = "http"
          type     = "http"
          path     = "/"
          interval = "10s"
          timeout  = "2s"
        }
      }

      task "app" {
        driver = "docker"

        # Compass uses this workload identity to call Nomad.
        identity {
          env = true
        }

        config {
          image = "ghcr.io/brianmichel/nomad-compass:v0.0.7"
          ports = ["http"]

          mount {
            type   = "volume"
            source = "compass-data"
            target = "/data"
          }
        }

        template {
          data = <<EOF
COMPASS_HTTP_ADDR = ":8080"
COMPASS_DATABASE_PATH = "/data/nomad-compass.sqlite"
COMPASS_REPO_BASE_DIR = "/data/repos"
COMPASS_NOMAD_ADDR = "http://100.74.142.86:4646"
COMPASS_CREDENTIAL_KEY = "{{ with nomadVar "nomad/jobs/compass" }}{{ .credential_key }}{{ end }}"
EOF

          destination = "secrets/env_vars"
          env         = true
        }

        resources {
          cpu    = 500
          memory = 100
        }
      }
    }
  }
}
