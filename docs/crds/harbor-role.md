# HarborRole CRD

`HarborRole` manages a custom role through Harbor's `/api/v2.0/roles` API. It is
cluster-scoped because Harbor roles belong to a Harbor instance, not a
Kubernetes namespace. Use a `ClusterHarborConnection` reference.

```yaml
apiVersion: harbor.harbor-entity-operator.io/v1alpha1
kind: HarborRole
metadata:
  name: release-reader
spec:
  harborConnectionRef:
    name: production-harbor
    kind: ClusterHarborConnection
  description: Read release metadata
  permissions:
    - kind: project-role
      namespace: "*"
      access:
        - resource: member
          action: pull
          effect: allow
```

The example grants read access to role members. Choose only the Harbor permissions
needed for the role. Built-in Harbor roles cannot be adopted, updated, or
deleted by this resource. `creationPolicy` follows the shared
[creation and deletion rules](../reference/deletion-and-ownership.md).

Harbor must expose the `/api/v2.0/roles` endpoints for this resource to
reconcile.
