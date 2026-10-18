# Project kickoff

Harbor Entity Operator manages an existing Harbor installation through Kubernetes custom resources. Harbor's official v2 API is the authority for endpoint behavior; this repository's CRDs add Kubernetes ownership, reference, status, and lifecycle contracts.

## Before changing the API

1. Read the relevant resource guide, Go API type, reconciler, Harbor client method, and tests.
2. Check the matching endpoint and schema in `hack/harbor-openapi.yaml`; record upstream changes in the API coverage review.
3. Evaluate Harbor identity and scope, connection and Secret references, tenant boundaries, adoption, deletion, finalizers, drift, dependencies, status, and current endpoint behavior.
4. Record consequential API or lifecycle choices in an ADR. Ask for guidance only when a remaining choice materially changes the public API, security boundary, or external deletion risk.

## Implementing a Harbor resource

Carry a resource change through its complete contract:

1. Add or update API types, validation, defaults, status, and print columns.
2. Add typed Harbor client operations with HTTP contract coverage.
3. Implement reconciliation, dependency handling, lifecycle behavior, status, and retries.
4. Add observable controller tests, a sample, a resource guide, and architecture notes.
5. Regenerate DeepCopy code, CRDs, RBAC, chart assets, and the API reference.
6. Run `make check`; use the local Kind stack for changes whose behavior depends on Harbor or a live Kubernetes cluster.

Never edit generated CRDs, RBAC, DeepCopy files, or API reference output directly. Keep credentials and local cluster state out of tracked files.
