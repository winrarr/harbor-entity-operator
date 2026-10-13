# Architecture

Harbor Entity Operator is a controller-runtime application that turns Kubernetes desired state into Harbor API v2 operations.

```text
Kubernetes custom resources and Secrets
                 ↓
      controller-runtime reconcilers
                 ↓
 Harbor connection resolution and client
                 ↓
          Harbor API v2.0
```

The public API under `api/v1alpha1` defines the CRDs. `internal/controller` owns reference resolution, lifecycle, finalizers, status, and drift handling. `internal/harborclient` contains Harbor HTTP contracts, pagination, and error handling. `charts/harbor-entity-operator` and `config/` are generated or packaged installation surfaces. `hack/harbor-openapi.yaml` is the official Harbor API reference used during implementation; CRDs are designed in Go and are not generated mechanically from the Swagger document.

The operator manages Harbor's control plane, not the registry data plane. Harbor remains authoritative for artifacts, repository contents, scans, and one-shot jobs. A Harbor custom resource records desired configuration, while status records Harbor identities and conditions needed for subsequent reconciliation.

Every Harbor-backed resource selects a namespaced `HarborConnection` or cluster-scoped `ClusterHarborConnection`. Namespaced references are checked against the operator's cross-namespace policy. Secrets hold credentials and generated robot outputs. Cluster-wide Harbor APIs use singleton ownership checks so multiple Kubernetes resources cannot silently overwrite the same Harbor setting.

Named Harbor objects generally use `metadata.name` as their remote identity. Resources that manage a relationship use Kubernetes references and the resolved Harbor IDs of their dependencies. Finalizers preserve cleanup obligations; `deletionPolicy: Orphan` releases those obligations without deleting the remote object. Existing-object behavior is controlled by the resource's creation policy.

For the concrete resource graph and boundaries, see [Technical Architecture](technical-architecture.md). For the upstream endpoint mapping and coverage decisions, see [Harbor API coverage](research/harbor-api-coverage.md).
