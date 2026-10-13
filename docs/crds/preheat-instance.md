# PreheatInstance CRD

`PreheatInstance` registers a Harbor P2P provider through
`/api/v2.0/p2p/preheat/instances`. It is cluster-scoped and requires a
`ClusterHarborConnection`.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: dragonfly-auth
  namespace: harbor-system
type: Opaque
stringData:
  token: replace-with-provider-credential
---
apiVersion: harbor.harbor-entity-operator.io/v1alpha1
kind: PreheatInstance
metadata:
  name: dragonfly
spec:
  harborConnectionRef:
    name: production-harbor
    kind: ClusterHarborConnection
  vendor: dragonfly
  endpoint: https://dragonfly.example.com
  authMode: BASIC
  authInfoSecretRef:
    name: dragonfly-auth
    namespace: harbor-system
  enabled: true
  default: false
```

The Secret's `data` keys and values become Harbor's `auth_info`. Keep credentials
in a Secret; they are not included in status or logs. Secret changes trigger a
reconcile. When authentication is disabled, set `authMode: NONE` and remove the
Secret reference so Harbor clears previously stored credentials. A non-`NONE`
auth mode requires a Secret with at least one data entry.

Use a vendor ID advertised by Harbor's preheat provider API. Harbor stores the
provider registration even if its endpoint is unhealthy; the observed health
value is exposed in `status.harborStatus`.

Delete or move all `PreheatPolicy` references before deleting a provider or
setting `enabled: false`. The operator blocks deletion and Harbor rejects a
disable update while policies still use the provider.
