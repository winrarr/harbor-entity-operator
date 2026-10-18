# Harbor API coverage

This review uses Harbor's official Swagger 2.0 document, `api/v2.0/swagger.yaml`, from [`goharbor/harbor`](https://github.com/goharbor/harbor/blob/35dd9e4337ca6857deffab19d28cc43df596f328/api/v2.0/swagger.yaml). The exact upstream revision is recorded in `hack/harbor-openapi.revision`.

The CRD boundary is Harbor control-plane desired state with a stable identity or singleton configuration endpoint. Harbor's REST API also contains read-only queries, artifact data operations, and imperative job operations; those do not automatically become Kubernetes resources.

## Supported resources

| Kubernetes resource | Harbor API surface |
| --- | --- |
| `HarborConnection`, `ClusterHarborConnection` | Operator connection settings; no Harbor resource endpoint |
| `Configuration` | `PUT /configurations` |
| `Project` | `/projects`, project properties, metadata, and project CVE allowlist endpoints |
| `User` | `/users` and user profile updates |
| `UserGroupClaim` | `/usergroups`; non-owning claim of a Harbor-global group identity |
| `HarborUserGroup` | `/usergroups`; owns a Harbor-global group |
| `HarborRole` | `/roles`; owns a custom Harbor role |
| `Member` | `/projects/{project}/members` |
| `Robot` | `/robots` |
| `Registry` | `/registries` |
| `ReplicationPolicy` | `/replication/policies` |
| `RetentionPolicy` | `/retentions` |
| `Quota` | `/quotas/{id}` |
| `GCSchedule` | `/system/gc/schedule` |
| `PurgeAuditSchedule` | `/system/purgeaudit/schedule` |
| `ScanAllSchedule` | `/system/scanAll/schedule` |
| `WebhookPolicy` | `/projects/{project}/webhook/policies` |
| `ScannerRegistration` | `/scanners` |
| `ImmutableTagRule` | `/projects/{project}/immutabletagrules` |
| `Label` | `/labels` and project label operations |
| `HarborCVEAllowlist` | `/system/CVEAllowlist`; singleton system configuration |
| `PreheatInstance` | `/p2p/preheat/instances`; Harbor-global provider registration |
| `PreheatPolicy` | `/projects/{project_name}/preheat/policies`; project policy |

The Harbor connection resources are operator configuration. The other rows are reconciled through Harbor's v2 API, with their ownership, reference, and deletion contracts described in the corresponding resource guides.

## Deliberate boundary

- Health, ping, search, statistics, audit-log reads, scanner tests, and permission-list endpoints are observations or validation operations. They are not persistent desired state.
- Artifact, repository, tag, scan, and artifact-label endpoints operate on registry content. Harbor's registry clients and promotion pipelines remain the source of truth for that data.
- Replication, retention, scan, garbage-collection, purge, preheat, and job-service execution endpoints trigger or stop individual jobs. Schedule and policy CRDs manage their persistent configuration; individual runs remain imperative actions.
- LDAP user import and OIDC/LDAP connectivity tests are one-time identity workflows. Harbor or the identity provider remains authoritative for those users and groups.

The custom role, user-group, system CVE allowlist, and preheat endpoints are represented by the CRDs above. Role, group, and provider identities are cluster-scoped; the system allowlist has one owner per Harbor instance; project preheat policies depend on a managed Project and PreheatInstance. Preheat credentials use Secret references. Individual preheat executions remain imperative operations.

The checked-in OpenAPI snapshot follows Harbor's current `main` branch and includes the `/roles` API managed by `HarborRole`.

Harbor's user-group update endpoint does not apply `group_type` changes, so `HarborUserGroup.spec.groupType` is immutable. Harbor also rejects disabling a preheat provider while any policy references it; remove or move those policies first.
