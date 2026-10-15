# Robot CRD

A **Robot** custom resource manages Harbor robot accounts using the
`/api/v2.0/robots` API. It supports both system- and project-level robots.

## Quick Start

```yaml
apiVersion: harbor.harbor-entity-operator.io/v1alpha1
kind: Robot
metadata:
  name: ci
spec:
  harborConnectionRef:
    name: my-harbor
    kind: HarborConnection
  creationPolicy: Create
  level: project
  permissions:
  - kind: project
    projectRef:
      name: library
    access:
    - resource: repository
      action: pull
      effect: allow
    - resource: repository
      action: push
      effect: allow
  secretRef:
    name: harbor-robot-ci
    key: secret
  imagePullSecretRef:
    name: harbor-robot-ci-pull
    labels:
      operator.example.io/secret-consumer: tenant-a
```

## Key Fields

At least one of `spec.secretRef` and `spec.imagePullSecretRef` is required.
Both fields may be set when consumers need both Secret formats.

- **spec.harborConnectionRef** (object, optional when the operator is configured with `--harbor-connection`)
  Reference to the Harbor connection object to use. Set `name` and optional `kind` (`HarborConnection` by default or `ClusterHarborConnection`).

- **spec.level** (string, required)
  Robot scope. Must be `system` or `project`.

- **spec.permissions** (array, required)
  Permissions granted to the robot. Each permission includes a `kind`, an
  optional `projectRef`, and one or more access rules. For `kind: project`, a
  `projectRef` resolves the Harbor project name through the Project CR. If it is
  omitted, the operator uses Harbor's native all-projects scope (`*`). For
  `kind: system`, the operator uses Harbor's system scope (`/`). Each access
  rule's `effect` defaults to `allow`.

- **spec.disable** (bool, optional)
  Controls whether the robot is disabled. When omitted, the operator leaves the
  value unset during creation and preserves the current value during updates.

- **spec.duration** (int, optional)
  Duration in days. Use `-1` for never expires. If omitted, it defaults to `-1`.

- **spec.secretRef** (object, optional)
  Reference to the operator-managed secret where the generated robot secret is written.
  The Secret contains the token under `secret` by default and Harbor's canonical
  robot username under `username`; that key is reserved and cannot be selected
  for the token. Optional `namespace`, `labels`, and `annotations` select the
  destination namespace and metadata. When `labels` or `annotations` change, the
  operator removes metadata keys previously set through this reference while
  preserving unrelated metadata and its reserved ownership metadata.
  If the Secret already exists, it must already be managed by the same `Robot`.

- **spec.imagePullSecretRef** (object, optional)
  References a separate operator-managed Secret containing a
  `kubernetes.io/dockerconfigjson` image-pull credential for the resolved Harbor
  connection. The generated configuration uses the Robot's current canonical
  username and token, and defaults to the Robot namespace. Optional `labels` and
  `annotations` are reconciled using the same merge and cleanup behavior as
  `secretRef`. The field has a fixed data shape and does not accept `key`.

  When `spec.secretRef` is omitted, this Secret is the Robot's sole credential
  output and is also used as the credential source during later reconciles.

  The image-pull Secret must be different from `spec.secretRef`. If a namespace
  is specified, the operator's cross-namespace reference setting applies.

- **spec.creationPolicy** (string, optional)
  Controls whether the robot is created, adopted, or either. When omitted, uses the operator's default creation policy (`Create` unless configured otherwise).

Project-scoped Robot names are persisted by Harbor as the project name, a `+`
separator, and the requested Robot name. The combined name must be at most 255
characters. Reconciliation reports a Ready error before sending a create
request when the name would exceed that limit; it never truncates or rewrites
`metadata.name`. System-scoped Robots use `metadata.name` directly and are not
subject to the combined project-name check.

Robot secrets are rotated automatically once Harbor reports that the robot
credential has expired (based on `expires_at`). The operator then refreshes the
secret and stores it in the configured output Secret or Secrets.

## Common Fields

`Robot` embeds `HarborSpecBase`. See [Common Spec Fields](../reference/common-spec-fields.md)
for the shared connection, deletion, and reconciliation controls, or jump to the
generated [`HarborSpecBase` reference](../reference/api.md#harborspecbase).

## Behavior

- **Create**

  - Creates the robot account with the requested permissions.
  - Uses `metadata.name` as the Harbor robot name.
  - Records Harbor's canonical username in `status.username` and the managed Secret.
  - Applies `creationPolicy` when the robot is not yet recorded in status.

- **Update**

  - Updates description, permissions, disabled state, and duration.
  - Keeps `status.username` and the managed Secret's `username` key synchronized with Harbor.
  - Rotates the Harbor credential when Harbor reports the current secret as expired.
  - Writes the rotated value back to the configured output Secret or Secrets.

- **Delete**

  - Deletes the robot account in Harbor.
  - Leaves the operator-managed Kubernetes Secret in place; remove that Secret
    separately when it is no longer needed.

- **Image-pull credentials**

  - When `spec.imagePullSecretRef` is set, the operator keeps the referenced
    `kubernetes.io/dockerconfigjson` Secret synchronized with the current Robot
    credentials.
  - The image-pull Secret is also left in place when the `Robot` is deleted;
    remove it separately when it is no longer needed.

## Notes

- `spec.secretRef` is a destination for operator-managed output, not an input source.
- `spec.imagePullSecretRef` is a Kubernetes image-pull Secret output. If it is the
  only configured output, the operator reads the current token from its managed
  Docker configuration when reconciling the Robot.
- The controller does not adopt or overwrite unrelated existing Secrets.

## Distribute an image-pull Secret with Capsule

Set a selector label on the generated image-pull Secret and use that label from
a Capsule `TenantResource`. The Secret can live in a shared namespace; Capsule
selects it by label and propagates it into the tenant's namespaces.

```yaml
apiVersion: harbor.harbor-entity-operator.io/v1alpha1
kind: Robot
metadata:
  name: ci
  namespace: shared-secrets
spec:
  level: project
  permissions:
    - kind: project
      projectRef:
        name: library
      access:
        - resource: repository
          action: pull
  imagePullSecretRef:
    name: harbor-robot-ci-pull
    labels:
      operator.example.io/secret-consumer: tenant-a
---
apiVersion: capsule.clastix.io/v1beta2
kind: TenantResource
metadata:
  name: harbor-robot-pull-secrets
  namespace: tenant-a
spec:
  resources:
    - namespacedItems:
        - apiVersion: v1
          kind: Secret
          selector:
            matchLabels:
              operator.example.io/secret-consumer: tenant-a
```

The operator continues to own the Secret's data and ownership metadata. Capsule
copies matching Secrets to namespaces belonging to the `TenantResource` owner.
