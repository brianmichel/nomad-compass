variable "dc" {
  type    = string
  default = "*"
}

variable "region" {
  type    = string
  default = "global"
}

variable "image" {
  type    = string
  default = "nomad-compass:1.0.0"
}

variable "nomad_addr" {
  type    = string
  default = "http://host.docker.internal:4646"
}

variable "credential_key" {
  type    = string
  default = "<replace_with_your_credential_key>"
}

job "compass" {
  datacenters = [var.dc]
  region      = var.region
  namespace = "system"

  group "compass" {
    count = 1

    network {
      port "http" {
        to = 8080
      }
    }

    service {
      name     = "compass"
      provider = "nomad"
      port     = "http"
      tags     = ["ui"]

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

      identity {
        env = true
      }

      config {
        image = var.image
        ports = ["http"]
      }

      env {
        COMPASS_HTTP_ADDR     = ":8080"
        COMPASS_DATABASE_PATH = "/data/nomad-compass.sqlite"
        COMPASS_REPO_BASE_DIR = "/data/repos"
        # For local testing make sure the Nomad address is resolvable from within your container.
        # Keep the credential encryption key in Nomad Variables or another secrets backend in production.
        # NOMAD_TOKEN is supplied by the task workload identity configured above.
        COMPASS_NOMAD_ADDR     = var.nomad_addr
        COMPASS_CREDENTIAL_KEY = var.credential_key
      }

      resources {
        cpu    = 500
        memory = 100
      }
    }
  }
}
