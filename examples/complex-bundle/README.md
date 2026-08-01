# Complex bundle example

`compass.bundle.hcl` is a review and validation fixture rather than a
ready-to-run production deployment. It demonstrates the resource graph used
by Compass:

```text
namespace + variable + volume + ACL policy
                    \_______________________/
                              job.api
```

The job depends on every supporting resource, mounts the managed host volume,
and declares a managed variable for application configuration. Destructive resources
use `delete = "protect"` so a reconcile cannot remove them accidentally.

Quota and Sentinel policy adapters are implemented too, but their Nomad
endpoints require Nomad Enterprise and are intentionally not included in this
OSS-compatible fixture.

Validate it offline:

```bash
go run ./cmd/nomad-compass bundle validate \
  --file examples/complex-bundle/compass.bundle.hcl
```

Generate a plan against a modified copy:

```bash
go run ./cmd/nomad-compass bundle plan \
  --file examples/complex-bundle/compass.bundle.hcl \
  --against examples/complex-bundle/compass.bundle.hcl \
  --revision example
```

The image and datacenter are illustrative. Replace them before applying this
bundle to a Nomad cluster.
