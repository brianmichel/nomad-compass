variable "dc1" {
  type    = string
  default = "dc1"
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

variable "nomad_token" {
  type    = string
  default = "<your_nomad_token_here>"
}

variable "credential_key" {
  type    = string
  default = "<replace_with_your_credential_key>"
}

job "compass" {
  datacenters = [var.dc1]
  region      = var.region

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

      config {
        image = var.image
        ports = ["http"]
      }

      env {
        COMPASS_HTTP_ADDR     = ":8080"
        COMPASS_DATABASE_PATH = "/data/nomad-compass.sqlite"
        COMPASS_REPO_BASE_DIR = "/data/repos"
        # For local testing make sure the Nomad address is resolvable from within your container.
        # Ideally replace the below three environment variables with Nomad or Vault secrets.
        COMPASS_NOMAD_ADDR     = var.nomad_addr
        COMPASS_NOMAD_TOKEN    = var.nomad_token
        COMPASS_CREDENTIAL_KEY = var.credential_key
      }

      resources {
        cpu    = 500
        memory = 100
      }
    }
  }
}
