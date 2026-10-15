# PreheatPolicy CRD

`PreheatPolicy` manages one Harbor project preheat policy through
`/api/v2.0/projects/{project_name}/preheat/policies`. It is namespaced. The
referenced `Project` and cluster-scoped `PreheatInstance` must be Ready and
target the same Harbor instance.

```yaml
apiVersion: harbor.harbor-entity-operator.io/v1alpha1
kind: PreheatPolicy
metadata:
  name: all-artifacts
  namespace: harbor-system
spec:
  harborConnectionRef:
    name: production-harbor
    kind: ClusterHarborConnection
  projectRef:
    name: releases
  providerRef:
    name: dragonfly
  filters: '[{"type":"resource","value":"artifact"},{"type":"name","value":"**"}]'
  trigger: '{"type":"manual","trigger_settings":{}}'
  extraAttrs: '{}'
  enabled: false
```

`filters` is a JSON array, while `trigger` and `extraAttrs` are JSON objects;
the controller validates these shapes before calling Harbor. The values remain
strings in the CR to match Harbor's API payload. `providerRef` is an object with
a required `name` that references a cluster-scoped `PreheatInstance`. Deleting the policy removes it
from Harbor. A `Project` cannot be deleted while any `PreheatPolicy` still
references it, and a `PreheatInstance` cannot be deleted while referenced by a
policy.
