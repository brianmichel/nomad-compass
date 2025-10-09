client {
  enabled = true
}

server {
  enabled          = true
  bootstrap_expect = 1
}

acl {
  enabled = true
}

plugin "docker" {
  config {
    # If you use something like Colima on macOS, you will have to adjust the docker endpoint below.
    # endpoint = "unix:///Users/<username>/.colima/default/docker.sock"
  }
}
