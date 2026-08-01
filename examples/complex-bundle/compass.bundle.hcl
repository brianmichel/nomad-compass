# A deliberately dense Compass bundle for validation, planning, and review.
# It demonstrates the OSS-compatible managed resource adapters plus a job that
# consumes the managed volume and namespace while declaring a managed variable.
# Quotas and Sentinel policies have separate adapters but require Nomad
# Enterprise endpoints.

bundle "platform" {
  resource "namespace" "apps" {
    description = "Application workloads managed by Compass"
  }

  resource "variable" "apps/config" {
    namespace = "apps"

    items = {
      environment = "production"
      log_level   = "info"
    }
  }

  resource "volume" "app_data" {
    delete = "protect"

    name      = "platform-app-data"
    namespace = "apps"
    type      = "host"
    plugin_id = "mkdir"

    capability {
      access_mode     = "single-node-single-writer"
      attachment_mode = "file-system"
    }
  }

  resource "acl_policy" "platform" {
    delete = "protect"

    description = "Read and submit access for platform workloads"

    rules {
      namespace "apps" {
        capabilities = [
          "read-job",
          "submit-job",
          "read-logs",
        ]

        variables {
          path "apps/config" {
            capabilities = ["read"]
          }
        }
      }
    }
  }

  resource "job" "api" {
    depends_on = [
      "namespace.apps",
      "variable.apps/config",
      "volume.app_data",
      "acl_policy.platform",
    ]

    namespace   = "apps"
    datacenters = ["dc1"]
    type        = "service"

    group "api" {
      count = 1

      volume "app-data" {
        type            = "host"
        source          = "platform-app-data"
        access_mode     = "single-node-single-writer"
        attachment_mode = "file-system"
        sticky          = true
      }

      network {
        port "http" {
          to = 8080
        }
      }

      service {
        name     = "platform-api"
        provider = "nomad"
        port     = "http"
      }

      task "server" {
        driver = "docker"

        identity {
          env = true
        }

        config {
          image   = "busybox:1.36"
          command = "sh"
          args    = ["-c", "test -d /var/lib/platform && echo complex-bundle-ok && sleep 3600"]
        }

        volume_mount {
          volume      = "app-data"
          destination = "/var/lib/platform"
          read_only   = false
        }

        resources {
          cpu    = 1
          memory = 32
        }
      }
    }
  }
}
