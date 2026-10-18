# User stories

## Current stories

- As a platform operator, I want to connect the controller to an existing Harbor instance using Kubernetes Secrets so credentials stay out of manifests.
- As a platform operator, I want Harbor projects, users, robots, registries, and policy resources represented as Kubernetes objects so configuration can be reviewed and versioned with applications.
- As a team owner, I want project members and dependent policies to reference other Kubernetes resources so identity and project relationships remain understandable.
- As a maintainer, I want adoption, deletion, orphaning, drift, and connection-loss behavior to be explicit so reconciliation does not make unexpected changes in Harbor.
- As an operator maintainer, I want Harbor's official OpenAPI document checked in with its upstream revision so client behavior and CRD coverage can be reviewed against the source contract.
- As an operator maintainer, I want generated CRDs, RBAC, Helm assets, API reference docs, unit tests, contract tests, and Kind checks to agree.
- As a platform operator, I want namespace scoping, connection references, and Secret access to respect the configured tenancy boundary.

## Future stories

- As a platform operator, I want to manage additional Harbor control-plane features as they gain stable desired-state semantics in Harbor's API.
- As an application owner, I want the operator to publish or copy OCI artifacts only if a separate content-delivery workflow can define safe source, provenance, and deletion semantics.
