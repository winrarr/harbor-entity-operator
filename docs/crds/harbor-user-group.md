# HarborUserGroup CRD

`HarborUserGroup` manages a Harbor-global group using the `/api/v2.0/usergroups`
API. It is cluster-scoped and must reference a `ClusterHarborConnection`.

```yaml
apiVersion: harbor.harbor-entity-operator.io/v1alpha1
kind: HarborUserGroup
metadata:
  name: developers
spec:
  harborConnectionRef:
    name: production-harbor
    kind: ClusterHarborConnection
  groupType: 2
```

Harbor uses group type `1` for LDAP, `2` for HTTP, and `3` for OIDC. LDAP groups
must also set `ldapGroupDN`. Use `UserGroupClaim` when Harbor or the identity
provider owns the group and Kubernetes should only refer to it. A managed group
cannot be deleted while a matching `UserGroupClaim` targets the same Harbor
instance. `groupType` is immutable after creation because Harbor's update
endpoint returns success without applying a group type change.

`creationPolicy` controls creation and adoption. Deletion uses the common
`deletionPolicy` contract and defaults to deleting the Harbor group.
