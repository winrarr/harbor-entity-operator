# Robot Credentials

Robot accounts are a good example of the operator owning generated output.

## Robot

```yaml
apiVersion: harbor.harbor-entity-operator.io/v1alpha1
kind: Robot
metadata:
  name: ci
spec:
  harborConnectionRef:
    name: my-harbor
    kind: HarborConnection
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

## Notes

- at least one output reference is required
- the operator creates and manages each configured output Secret
- when requested, it also creates and manages a separate
  `kubernetes.io/dockerconfigjson` image-pull Secret
- when `imagePullSecretRef` is the only output, that Secret is also used as the
  credential source during later reconciles
- if the target secret already exists and is unrelated, reconciliation fails instead of silently adopting it
- labels and annotations on generated Secret outputs are reconciled without replacing the controller's ownership metadata

## Distribute an image-pull Secret with Capsule

The output label can select the generated Secret for Capsule replication. The
source Secret may live in a shared namespace; `TenantResource` selects it by
label and distributes it to namespaces in the tenant.

```yaml
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
