# Research

Research notes record upstream evidence that affects the Kubernetes API, Harbor client, reconciliation, and compatibility decisions.

- [Harbor API coverage](harbor-api-coverage.md) maps supported CRDs and deliberately excluded API operations to the checked-in official OpenAPI snapshot.
- [Reference operators](reference-operators.md) records the engineering sources used alongside Harbor's own API contract.

The OpenAPI snapshot is stored at `hack/harbor-openapi.yaml`; `hack/harbor-openapi.revision` records the exact `goharbor/harbor` commit. Refresh it with `make update-harbor-openapi` and review the resulting API differences before changing CRDs or client behavior.
