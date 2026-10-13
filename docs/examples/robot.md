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
```

## Notes

- at least one output reference is required
- the operator creates and manages each configured output Secret
- when requested, it also creates and manages a separate
  `kubernetes.io/dockerconfigjson` image-pull Secret
- when `imagePullSecretRef` is the only output, that Secret is also used as the
  credential source during later reconciles
- if the target secret already exists and is unrelated, reconciliation fails instead of silently adopting it
