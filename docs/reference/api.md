# API Reference

## Packages
- [harbor.harbor-entity-operator.io/v1alpha1](#harborharbor-entity-operatoriov1alpha1)


## harbor.harbor-entity-operator.io/v1alpha1

Package v1alpha1 contains API Schema definitions for the harbor v1alpha1 API group.

### Resource Types
- [ClusterHarborConnection](#clusterharborconnection)
- [Configuration](#configuration)
- [GCSchedule](#gcschedule)
- [HarborCVEAllowlist](#harborcveallowlist)
- [HarborConnection](#harborconnection)
- [HarborRole](#harborrole)
- [HarborUserGroup](#harborusergroup)
- [ImmutableTagRule](#immutabletagrule)
- [Label](#label)
- [Member](#member)
- [PreheatInstance](#preheatinstance)
- [PreheatPolicy](#preheatpolicy)
- [Project](#project)
- [PurgeAuditSchedule](#purgeauditschedule)
- [Quota](#quota)
- [Registry](#registry)
- [ReplicationPolicy](#replicationpolicy)
- [RetentionPolicy](#retentionpolicy)
- [Robot](#robot)
- [ScanAllSchedule](#scanallschedule)
- [ScannerRegistration](#scannerregistration)
- [User](#user)
- [UserGroupClaim](#usergroupclaim)
- [WebhookPolicy](#webhookpolicy)



#### CVEAllowlist



CVEAllowlist defines the writable project CVE allowlist configuration.



_Appears in:_
- [ProjectSpec](#projectspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `expiresAt` _integer_ | ExpiresAt is the optional expiration time as seconds since the Unix epoch.<br />Omit it for a non-expiring allowlist. |  | Minimum: 0 <br />Optional: \{\} <br /> |
| `items` _[CVEAllowlistItem](#cveallowlistitem) array_ | Items is the complete desired list of CVE identifiers. |  |  |


#### CVEAllowlistItem



CVEAllowlistItem defines a single CVE allowlist entry.



_Appears in:_
- [CVEAllowlist](#cveallowlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cveID` _string_ | CveID is the CVE identifier, such as CVE-2024-12345. |  | MinLength: 1 <br /> |


#### ClusterHarborConnection



ClusterHarborConnection is the Schema for the clusterharborconnections API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `ClusterHarborConnection` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[HarborConnectionSpec](#harborconnectionspec)_ |  |  |  |


#### Configuration



Configuration is the Schema for the configurations API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `Configuration` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ConfigurationSpec](#configurationspec)_ |  |  |  |


#### ConfigurationSpec



ConfigurationSpec defines the desired state of Harbor system configuration.



_Appears in:_
- [Configuration](#configuration)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls what happens when the Kubernetes object is deleted.<br />Delete removes the corresponding Harbor resource before removing the finalizer.<br />Orphan skips Harbor-side deletion and removes the finalizer so the<br />Kubernetes object can be deleted while leaving the Harbor resource in place.<br />Defaults to Delete. | Delete | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift.<br />When omitted, the operator's default drift detection interval is used.<br />An explicit value of 0 disables periodic drift detection. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `settings` _object (keys:string, values:[ConfigurationValue](#configurationvalue))_ | Settings contains Harbor configuration keys and their desired value sources. |  | Optional: \{\} <br /> |


#### ConfigurationValue



ConfigurationValue selects a literal or Secret-backed Harbor configuration
value.

_Validation:_
- MaxProperties: 1
- MinProperties: 1

_Appears in:_
- [ConfigurationSpec](#configurationspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `value` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#json-v1-apiextensions-k8s-io)_ | Value is the literal configuration value. |  | Optional: \{\} <br /> |
| `valueFrom` _[ConfigurationValueSource](#configurationvaluesource)_ | ValueFrom selects an external source for the configuration value. |  | Optional: \{\} <br /> |


#### ConfigurationValueSource



ConfigurationValueSource selects an external source for a Harbor
configuration value.



_Appears in:_
- [ConfigurationValue](#configurationvalue)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretKeyRef` _[SecretReference](#secretreference)_ | SecretKeyRef references the Secret key containing the configuration value. |  |  |


#### CreationPolicy

_Underlying type:_ _string_

CreationPolicy controls how the operator acquires a Harbor resource that is
not yet recorded in status.



_Appears in:_
- [HarborRoleSpec](#harborrolespec)
- [HarborUserGroupSpec](#harborusergroupspec)
- [ImmutableTagRuleSpec](#immutabletagrulespec)
- [LabelSpec](#labelspec)
- [MemberSpec](#memberspec)
- [PreheatInstanceSpec](#preheatinstancespec)
- [PreheatPolicySpec](#preheatpolicyspec)
- [ProjectSpec](#projectspec)
- [RegistrySpec](#registryspec)
- [ReplicationPolicySpec](#replicationpolicyspec)
- [RobotSpec](#robotspec)
- [ScannerRegistrationSpec](#scannerregistrationspec)
- [UserSpec](#userspec)
- [WebhookPolicySpec](#webhookpolicyspec)

| Field | Description |
| --- | --- |
| `Create` | CreationPolicyCreate creates a new Harbor resource and reports a conflict<br />when a matching resource already exists.<br /> |
| `Adopt` | CreationPolicyAdopt requires and adopts a matching Harbor resource without<br />creating one when no match exists.<br /> |
| `CreateOrAdopt` | CreationPolicyCreateOrAdopt adopts a matching Harbor resource when present<br />and creates one otherwise.<br /> |


#### Credentials



Credentials holds default authentication details.



_Appears in:_
- [HarborConnectionSpec](#harborconnectionspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _string_ | Type selects the credential type. Defaults to basic. | basic | Enum: [basic] <br />Optional: \{\} <br /> |
| `username` _string_ | Username for authentication. |  | MinLength: 1 <br />Optional: \{\} <br /> |
| `usernameSecretRef` _[SecretReference](#secretreference)_ | UsernameSecretRef points to the Kubernetes Secret that stores the username.<br />Use this when another controller, such as the Robot controller, produces the<br />canonical Harbor username. |  | Optional: \{\} <br /> |
| `passwordSecretRef` _[SecretReference](#secretreference)_ | PasswordSecretRef points to the Kubernetes Secret that stores the password / token. |  |  |


#### DeletionPolicy

_Underlying type:_ _string_





_Appears in:_
- [ConfigurationSpec](#configurationspec)
- [GCScheduleSpec](#gcschedulespec)
- [HarborCVEAllowlistSpec](#harborcveallowlistspec)
- [HarborRoleSpec](#harborrolespec)
- [HarborSpecBase](#harborspecbase)
- [HarborUserGroupSpec](#harborusergroupspec)
- [ImmutableTagRuleSpec](#immutabletagrulespec)
- [LabelSpec](#labelspec)
- [MemberSpec](#memberspec)
- [PreheatInstanceSpec](#preheatinstancespec)
- [PreheatPolicySpec](#preheatpolicyspec)
- [ProjectSpec](#projectspec)
- [PurgeAuditScheduleSpec](#purgeauditschedulespec)
- [QuotaSpec](#quotaspec)
- [RegistrySpec](#registryspec)
- [ReplicationPolicySpec](#replicationpolicyspec)
- [RetentionPolicySpec](#retentionpolicyspec)
- [RobotSpec](#robotspec)
- [ScanAllScheduleSpec](#scanallschedulespec)
- [ScannerRegistrationSpec](#scannerregistrationspec)
- [UserSpec](#userspec)
- [WebhookPolicySpec](#webhookpolicyspec)

| Field | Description |
| --- | --- |
| `Delete` |  |
| `Orphan` |  |


#### GCSchedule



GCSchedule is the Schema for the gcschedules API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `GCSchedule` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[GCScheduleSpec](#gcschedulespec)_ |  |  |  |


#### GCScheduleSpec



GCScheduleSpec defines the desired schedule for garbage collection.



_Appears in:_
- [GCSchedule](#gcschedule)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls what happens when the Kubernetes object is deleted.<br />Delete removes the corresponding Harbor resource before removing the finalizer.<br />Orphan skips Harbor-side deletion and removes the finalizer so the<br />Kubernetes object can be deleted while leaving the Harbor resource in place.<br />Defaults to Delete. | Delete | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift.<br />When omitted, the operator's default drift detection interval is used.<br />An explicit value of 0 disables periodic drift detection. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `schedule` _[ScheduleSpec](#schedulespec)_ | Schedule defines when GC runs. |  |  |
| `parameters` _object (keys:string, values:[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#json-v1-apiextensions-k8s-io))_ | Parameters define GC settings passed to Harbor. |  | Optional: \{\} <br /> |


#### HarborCVEAllowlist



HarborCVEAllowlist owns Harbor's system-wide CVE allowlist singleton.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `HarborCVEAllowlist` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[HarborCVEAllowlistSpec](#harborcveallowlistspec)_ |  |  |  |


#### HarborCVEAllowlistItem



HarborCVEAllowlistItem identifies a CVE excluded from Harbor vulnerability reports.



_Appears in:_
- [HarborCVEAllowlistSpec](#harborcveallowlistspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cveID` _string_ | CVEID is the CVE identifier, such as CVE-2024-12345. |  | MinLength: 1 <br /> |


#### HarborCVEAllowlistSpec



HarborCVEAllowlistSpec defines Harbor's system-wide CVE allowlist.



_Appears in:_
- [HarborCVEAllowlist](#harborcveallowlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor instance to configure. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls behavior when this singleton owner is deleted.<br />Delete clears the Harbor allowlist; Orphan leaves it unchanged. Defaults to Orphan. | Orphan | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval controls periodic drift checks. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `items` _[HarborCVEAllowlistItem](#harborcveallowlistitem) array_ | Items is the complete desired list of CVEs excluded from reports. |  | Optional: \{\} <br /> |
| `expiresAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | ExpiresAt is the optional expiration time for all entries. |  | Optional: \{\} <br /> |


#### HarborClaimSpecBase



HarborClaimSpecBase is the common spec for non-owning claims of Harbor
global identities. Claims select a connection and drift interval, but never
delete the global Harbor object when the claim is deleted.



_Appears in:_
- [UserGroupClaimSpec](#usergroupclaimspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |


#### HarborConnection



HarborConnection is the Schema for the harborconnections API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `HarborConnection` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[HarborConnectionSpec](#harborconnectionspec)_ |  |  |  |


#### HarborConnectionReference



HarborConnectionReference identifies either a namespaced HarborConnection or a
cluster-scoped ClusterHarborConnection.



_Appears in:_
- [ConfigurationSpec](#configurationspec)
- [GCScheduleSpec](#gcschedulespec)
- [HarborCVEAllowlistSpec](#harborcveallowlistspec)
- [HarborClaimSpecBase](#harborclaimspecbase)
- [HarborRoleSpec](#harborrolespec)
- [HarborSpecBase](#harborspecbase)
- [HarborUserGroupSpec](#harborusergroupspec)
- [ImmutableTagRuleSpec](#immutabletagrulespec)
- [LabelSpec](#labelspec)
- [MemberSpec](#memberspec)
- [PreheatInstanceSpec](#preheatinstancespec)
- [PreheatPolicySpec](#preheatpolicyspec)
- [ProjectSpec](#projectspec)
- [PurgeAuditScheduleSpec](#purgeauditschedulespec)
- [QuotaSpec](#quotaspec)
- [RegistrySpec](#registryspec)
- [ReplicationPolicySpec](#replicationpolicyspec)
- [RetentionPolicySpec](#retentionpolicyspec)
- [RobotSpec](#robotspec)
- [ScanAllScheduleSpec](#scanallschedulespec)
- [ScannerRegistrationSpec](#scannerregistrationspec)
- [UserGroupClaimSpec](#usergroupclaimspec)
- [UserSpec](#userspec)
- [WebhookPolicySpec](#webhookpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the referenced Harbor connection object. |  | MaxLength: 253 <br />MinLength: 1 <br />Pattern: `^[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?(\.[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?)*$` <br /> |
| `kind` _[HarborConnectionReferenceKind](#harborconnectionreferencekind)_ | Kind selects the Harbor connection object kind.<br />Defaults to HarborConnection. | HarborConnection | Enum: [HarborConnection ClusterHarborConnection] <br />Optional: \{\} <br /> |


#### HarborConnectionReferenceKind

_Underlying type:_ _string_





_Appears in:_
- [HarborConnectionReference](#harborconnectionreference)

| Field | Description |
| --- | --- |
| `HarborConnection` |  |
| `ClusterHarborConnection` |  |


#### HarborConnectionSpec



HarborConnectionSpec defines the desired state of HarborConnection.



_Appears in:_
- [ClusterHarborConnection](#clusterharborconnection)
- [HarborConnection](#harborconnection)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `baseURL` _string_ | BaseURL is the Harbor API endpoint.<br />It must be an absolute HTTP or HTTPS URL without query or fragment components. |  | Format: uri <br />Pattern: `^https?://[^/?#]+(/[^?#]*)?$` <br /> |
| `credentials` _[Credentials](#credentials)_ | Credentials holds the default credentials for Harbor API calls.<br />Omit this field to connect without authentication. |  | Optional: \{\} <br /> |
| `caBundle` _string_ | CABundle is a PEM-encoded CA bundle for validating Harbor TLS certificates. |  | Optional: \{\} <br /> |
| `caBundleSecretRef` _[SecretReference](#secretreference)_ | CABundleSecretRef references a Secret containing a PEM-encoded CA bundle.<br />When set, it is mutually exclusive with caBundle. |  | Optional: \{\} <br /> |


#### HarborRole



HarborRole manages a custom Harbor system role. Built-in roles cannot be managed.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `HarborRole` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[HarborRoleSpec](#harborrolespec)_ |  |  |  |


#### HarborRoleAccess



HarborRoleAccess is one resource/action/effect rule in a custom role.



_Appears in:_
- [HarborRolePermission](#harborrolepermission)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resource` _string_ | Resource is the Harbor resource name. |  | MinLength: 1 <br /> |
| `action` _string_ | Action is the Harbor action name, such as pull, push, read, or delete. |  | MinLength: 1 <br /> |
| `effect` _string_ | Effect is the permission effect. |  | MinLength: 1 <br /> |


#### HarborRolePermission



HarborRolePermission describes a permission granted by a custom Harbor role.



_Appears in:_
- [HarborRoleSpec](#harborrolespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is the Harbor permission category. |  | MinLength: 1 <br /> |
| `namespace` _string_ | Namespace scopes the permission to a Harbor resource namespace. |  | Optional: \{\} <br /> |
| `access` _[HarborRoleAccess](#harborroleaccess) array_ | Access contains the resource/action/effect rules. |  | MinItems: 1 <br /> |


#### HarborRoleSpec



HarborRoleSpec defines a custom, Harbor-global role.



_Appears in:_
- [HarborRole](#harborrole)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls what happens when the Kubernetes object is deleted.<br />Delete removes the corresponding Harbor resource before removing the finalizer.<br />Orphan skips Harbor-side deletion and removes the finalizer so the<br />Kubernetes object can be deleted while leaving the Harbor resource in place.<br />Defaults to Delete. | Delete | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift.<br />When omitted, the operator's default drift detection interval is used.<br />An explicit value of 0 disables periodic drift detection. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `creationPolicy` _[CreationPolicy](#creationpolicy)_ | CreationPolicy controls whether the role is created or adopted. |  | Enum: [Create Adopt CreateOrAdopt] <br />Optional: \{\} <br /> |
| `description` _string_ | Description is the role's human-readable description. |  | Optional: \{\} <br /> |
| `permissions` _[HarborRolePermission](#harborrolepermission) array_ | Permissions lists the role's Harbor API permission rules. |  | MinItems: 1 <br /> |


#### HarborSpecBase



HarborSpecBase holds the fields that appear in every Harbor CR.



_Appears in:_
- [ConfigurationSpec](#configurationspec)
- [GCScheduleSpec](#gcschedulespec)
- [HarborRoleSpec](#harborrolespec)
- [HarborUserGroupSpec](#harborusergroupspec)
- [ImmutableTagRuleSpec](#immutabletagrulespec)
- [LabelSpec](#labelspec)
- [MemberSpec](#memberspec)
- [PreheatInstanceSpec](#preheatinstancespec)
- [PreheatPolicySpec](#preheatpolicyspec)
- [ProjectSpec](#projectspec)
- [PurgeAuditScheduleSpec](#purgeauditschedulespec)
- [QuotaSpec](#quotaspec)
- [RegistrySpec](#registryspec)
- [ReplicationPolicySpec](#replicationpolicyspec)
- [RetentionPolicySpec](#retentionpolicyspec)
- [RobotSpec](#robotspec)
- [ScanAllScheduleSpec](#scanallschedulespec)
- [ScannerRegistrationSpec](#scannerregistrationspec)
- [UserSpec](#userspec)
- [WebhookPolicySpec](#webhookpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls what happens when the Kubernetes object is deleted.<br />Delete removes the corresponding Harbor resource before removing the finalizer.<br />Orphan skips Harbor-side deletion and removes the finalizer so the<br />Kubernetes object can be deleted while leaving the Harbor resource in place.<br />Defaults to Delete. | Delete | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift.<br />When omitted, the operator's default drift detection interval is used.<br />An explicit value of 0 disables periodic drift detection. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |


#### HarborUserGroup



HarborUserGroup manages a Harbor-global user group.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `HarborUserGroup` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[HarborUserGroupSpec](#harborusergroupspec)_ |  |  |  |


#### HarborUserGroupSpec



HarborUserGroupSpec defines a Harbor-global user group.



_Appears in:_
- [HarborUserGroup](#harborusergroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls what happens when the Kubernetes object is deleted.<br />Delete removes the corresponding Harbor resource before removing the finalizer.<br />Orphan skips Harbor-side deletion and removes the finalizer so the<br />Kubernetes object can be deleted while leaving the Harbor resource in place.<br />Defaults to Delete. | Delete | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift.<br />When omitted, the operator's default drift detection interval is used.<br />An explicit value of 0 disables periodic drift detection. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `creationPolicy` _[CreationPolicy](#creationpolicy)_ | CreationPolicy controls whether the group is created or adopted. |  | Enum: [Create Adopt CreateOrAdopt] <br />Optional: \{\} <br /> |
| `groupType` _integer_ | GroupType is 1 for LDAP, 2 for HTTP, and 3 for OIDC groups. Harbor does not<br />change this field through its update endpoint, so it is immutable. |  | Enum: [1 2 3] <br /> |
| `ldapGroupDN` _string_ | LDAPGroupDN is the LDAP distinguished name for LDAP-backed groups. |  | Optional: \{\} <br /> |


#### ImmutableSelector



ImmutableSelector defines an immutable tag rule selector.



_Appears in:_
- [ImmutableTagRuleSpec](#immutabletagrulespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind defines selector kind. |  | Optional: \{\} <br /> |
| `decoration` _string_ | Decoration defines selector decoration. |  | Optional: \{\} <br /> |
| `pattern` _string_ | Pattern defines selector pattern. |  | Optional: \{\} <br /> |
| `extras` _string_ | Extras defines extra selector details. |  | Optional: \{\} <br /> |


#### ImmutableTagRule



ImmutableTagRule is the Schema for the immutabletagrules API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `ImmutableTagRule` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ImmutableTagRuleSpec](#immutabletagrulespec)_ |  |  |  |


#### ImmutableTagRuleSpec



ImmutableTagRuleSpec defines the desired state of ImmutableTagRule.



_Appears in:_
- [ImmutableTagRule](#immutabletagrule)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls what happens when the Kubernetes object is deleted.<br />Delete removes the corresponding Harbor resource before removing the finalizer.<br />Orphan skips Harbor-side deletion and removes the finalizer so the<br />Kubernetes object can be deleted while leaving the Harbor resource in place.<br />Defaults to Delete. | Delete | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift.<br />When omitted, the operator's default drift detection interval is used.<br />An explicit value of 0 disables periodic drift detection. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `creationPolicy` _[CreationPolicy](#creationpolicy)_ | CreationPolicy controls whether the operator creates or adopts the Harbor rule.<br />When omitted, the operator's default creation policy is used. |  | Enum: [Create Adopt CreateOrAdopt] <br />Optional: \{\} <br /> |
| `projectRef` _[ProjectReference](#projectreference)_ | ProjectRef references a Project CR to derive the Harbor project ID. |  | Optional: \{\} <br /> |
| `disabled` _boolean_ | Disabled indicates whether the rule is disabled. Defaults to false. | false | Optional: \{\} <br /> |
| `action` _string_ | Action defines the rule action. |  | Optional: \{\} <br /> |
| `template` _string_ | Template defines the rule template. |  | Optional: \{\} <br /> |
| `params` _object (keys:string, values:[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#json-v1-apiextensions-k8s-io))_ | Params holds template parameters. |  | Optional: \{\} <br /> |
| `tagSelectors` _[ImmutableSelector](#immutableselector) array_ | TagSelectors define tag selectors. |  | Optional: \{\} <br /> |
| `scopeSelectors` _object (keys:string, values:[ImmutableSelector](#immutableselector))_ | ScopeSelectors define scope selectors. |  | Optional: \{\} <br /> |
| `priority` _integer_ | Priority defines the rule priority. |  | Optional: \{\} <br /> |


#### Label



Label is the Schema for the labels API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `Label` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[LabelSpec](#labelspec)_ |  |  |  |


#### LabelSpec



LabelSpec defines the desired state of Label.



_Appears in:_
- [Label](#label)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls what happens when the Kubernetes object is deleted.<br />Delete removes the corresponding Harbor resource before removing the finalizer.<br />Orphan skips Harbor-side deletion and removes the finalizer so the<br />Kubernetes object can be deleted while leaving the Harbor resource in place.<br />Defaults to Delete. | Delete | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift.<br />When omitted, the operator's default drift detection interval is used.<br />An explicit value of 0 disables periodic drift detection. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `creationPolicy` _[CreationPolicy](#creationpolicy)_ | CreationPolicy controls whether the operator creates or adopts the Harbor label.<br />When omitted, the operator's default creation policy is used. |  | Enum: [Create Adopt CreateOrAdopt] <br />Optional: \{\} <br /> |
| `description` _string_ | Description is an optional description. |  | Optional: \{\} <br /> |
| `color` _string_ | Color is the label color, e.g. #3366ff. |  | Optional: \{\} <br /> |
| `scope` _string_ | Scope is the label scope. Valid values are g (global) and p (project).<br />When omitted, it defaults to p if ProjectRef is set and g otherwise. |  | Enum: [g p] <br />Optional: \{\} <br /> |
| `projectRef` _[ProjectReference](#projectreference)_ | ProjectRef references a Project CR for project-scoped labels. |  | Optional: \{\} <br /> |


#### Member



Member is the Schema for the members API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `Member` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[MemberSpec](#memberspec)_ |  |  |  |


#### MemberGroup



MemberGroup defines a group-based member.



_Appears in:_
- [MemberSpec](#memberspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `groupClaimRef` _[UserGroupClaimReference](#usergroupclaimreference)_ | GroupClaimRef references the external group claim to grant membership to. |  |  |


#### MemberSpec



MemberSpec defines the desired state of Member.



_Appears in:_
- [Member](#member)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls what happens when the Kubernetes object is deleted.<br />Delete removes the corresponding Harbor resource before removing the finalizer.<br />Orphan skips Harbor-side deletion and removes the finalizer so the<br />Kubernetes object can be deleted while leaving the Harbor resource in place.<br />Defaults to Delete. | Delete | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift.<br />When omitted, the operator's default drift detection interval is used.<br />An explicit value of 0 disables periodic drift detection. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `creationPolicy` _[CreationPolicy](#creationpolicy)_ | CreationPolicy controls whether the operator creates or adopts the Harbor membership.<br />When omitted, the operator's default creation policy is used. |  | Enum: [Create Adopt CreateOrAdopt] <br />Optional: \{\} <br /> |
| `projectRef` _[ProjectReference](#projectreference)_ | ProjectRef references the project where the member should be added. |  |  |
| `role` _string_ | Role is the human‑readable name of the role.<br />Allowed values: "admin", "maintainer", "developer", "guest" |  | Enum: [admin maintainer developer guest] <br />Required: \{\} <br /> |
| `memberUser` _[MemberUser](#memberuser)_ | MemberUser defines the member if it is a user. |  | Optional: \{\} <br /> |
| `memberGroup` _[MemberGroup](#membergroup)_ | MemberGroup defines the member if it is a group. |  | Optional: \{\} <br /> |


#### MemberUser



MemberUser defines a user-based member.



_Appears in:_
- [MemberSpec](#memberspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `userRef` _[UserReference](#userreference)_ | UserRef references the User to grant membership to. |  |  |


#### PreheatInstance



PreheatInstance manages a Harbor-global preheat provider instance.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `PreheatInstance` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[PreheatInstanceSpec](#preheatinstancespec)_ |  |  |  |


#### PreheatInstanceReference



PreheatInstanceReference identifies a cluster-scoped PreheatInstance resource.



_Appears in:_
- [PreheatPolicySpec](#preheatpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the PreheatInstance resource. |  | MaxLength: 253 <br />MinLength: 1 <br />Pattern: `^[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?(\.[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?)*$` <br /> |


#### PreheatInstanceSpec



PreheatInstanceSpec defines a Harbor-global P2P preheat provider instance.



_Appears in:_
- [PreheatInstance](#preheatinstance)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls what happens when the Kubernetes object is deleted.<br />Delete removes the corresponding Harbor resource before removing the finalizer.<br />Orphan skips Harbor-side deletion and removes the finalizer so the<br />Kubernetes object can be deleted while leaving the Harbor resource in place.<br />Defaults to Delete. | Delete | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift.<br />When omitted, the operator's default drift detection interval is used.<br />An explicit value of 0 disables periodic drift detection. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `creationPolicy` _[CreationPolicy](#creationpolicy)_ | CreationPolicy controls whether the instance is created or adopted. |  | Enum: [Create Adopt CreateOrAdopt] <br />Optional: \{\} <br /> |
| `description` _string_ | Description describes the preheat provider. |  | Optional: \{\} <br /> |
| `vendor` _string_ | Vendor selects the Harbor preheat driver. |  | MinLength: 1 <br /> |
| `endpoint` _string_ | Endpoint is the provider service endpoint.<br />It must be an absolute HTTP or HTTPS URL without query or fragment components. |  | Format: uri <br />Pattern: `^https?://[^/?#]+(/[^?#]*)?$` <br /> |
| `authMode` _string_ | AuthMode is the authentication mode understood by the selected provider. | NONE | MinLength: 1 <br /> |
| `authInfoSecretRef` _[SecretObjectReference](#secretobjectreference)_ | AuthInfoSecretRef references a Secret whose data entries are sent as provider credentials.<br />Cluster-scoped instances must set the Secret namespace explicitly. |  | Optional: \{\} <br /> |
| `enabled` _boolean_ | Enabled controls whether Harbor uses this instance. |  |  |
| `default` _boolean_ | Default marks this as Harbor's default preheat instance. |  | Optional: \{\} <br /> |
| `insecure` _boolean_ | Insecure allows TLS connections to the provider without certificate verification. |  | Optional: \{\} <br /> |


#### PreheatPolicy



PreheatPolicy manages a Harbor project preheat policy.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `PreheatPolicy` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[PreheatPolicySpec](#preheatpolicyspec)_ |  |  |  |


#### PreheatPolicySpec



PreheatPolicySpec defines project-level preheat behavior.



_Appears in:_
- [PreheatPolicy](#preheatpolicy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls what happens when the Kubernetes object is deleted.<br />Delete removes the corresponding Harbor resource before removing the finalizer.<br />Orphan skips Harbor-side deletion and removes the finalizer so the<br />Kubernetes object can be deleted while leaving the Harbor resource in place.<br />Defaults to Delete. | Delete | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift.<br />When omitted, the operator's default drift detection interval is used.<br />An explicit value of 0 disables periodic drift detection. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `creationPolicy` _[CreationPolicy](#creationpolicy)_ | CreationPolicy controls whether the policy is created or adopted. |  | Enum: [Create Adopt CreateOrAdopt] <br />Optional: \{\} <br /> |
| `projectRef` _[ProjectReference](#projectreference)_ | ProjectRef identifies the Harbor Project resource this policy belongs to. |  |  |
| `providerRef` _[PreheatInstanceReference](#preheatinstancereference)_ | ProviderRef identifies the cluster-scoped PreheatInstance used by this policy. |  |  |
| `description` _string_ | Description describes the policy. |  | Optional: \{\} <br /> |
| `filters` _string_ | Filters is the JSON-encoded Harbor preheat filter expression. |  | Optional: \{\} <br /> |
| `trigger` _string_ | Trigger is the JSON-encoded Harbor preheat trigger configuration. |  | Optional: \{\} <br /> |
| `enabled` _boolean_ | Enabled controls whether Harbor applies this policy. |  |  |
| `extraAttrs` _string_ | ExtraAttrs is the JSON-encoded provider-specific configuration. |  | Optional: \{\} <br /> |


#### Project



Project is the Schema for the projects API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `Project` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ProjectSpec](#projectspec)_ |  |  |  |


#### ProjectMetadata



ProjectMetadata defines additional metadata for the project. Nil values leave
the corresponding Harbor fields unmanaged. Pointers allow explicit empty
values for fields where Harbor uses an empty string to clear a setting.
Project visibility is managed through ProjectSpec.Public.



_Appears in:_
- [ProjectSpec](#projectspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enable_content_trust` _string_ | EnableContentTrust blocks pulling unsigned images when enabled. |  | Enum: [true false] <br />Optional: \{\} <br /> |
| `enable_content_trust_cosign` _string_ | EnableContentTrustCosign blocks pulling images without a cosign signature when enabled. |  | Enum: [true false] <br />Optional: \{\} <br /> |
| `prevent_vul` _string_ | PreventVul blocks vulnerable images from running when enabled. |  | Enum: [true false] <br />Optional: \{\} <br /> |
| `severity` _string_ | Severity is the minimum severity threshold for blocking vulnerable images. |  | Enum: [none low medium high critical] <br />Optional: \{\} <br /> |
| `auto_scan` _string_ | AutoScan scans images automatically when they are pushed. |  | Enum: [true false] <br />Optional: \{\} <br /> |
| `auto_sbom_generation` _string_ | AutoSBOMGeneration generates SBOMs automatically when subject artifacts are pushed. |  | Enum: [true false] <br />Optional: \{\} <br /> |
| `reuse_sys_cve_allowlist` _string_ | ReuseSysCVEAllowlist makes the project use the system-wide CVE allowlist. |  | Enum: [true false] <br />Optional: \{\} <br /> |
| `retention_id` _string_ | RetentionID is the Harbor ID of the project's tag retention policy. |  | Optional: \{\} <br /> |
| `proxy_speed_kb` _string_ | ProxySpeedKB limits proxy-cache bandwidth in kilobits per second. |  | Optional: \{\} <br /> |
| `max_upstream_conn` _string_ | MaxUpstreamConn is the maximum number of upstream connections per artifact.<br />Use -1 for no limit. |  | Pattern: `^(-1\|[0-9]+)$` <br />Optional: \{\} <br /> |
| `proxy_cache_local_on_not_found` _string_ | ProxyCacheLocalOnNotFound serves a cached image when it has been removed upstream. |  | Enum: [true false] <br />Optional: \{\} <br /> |
| `proxy_referrer_api` _string_ | ProxyReferrerAPI enables proxying OCI 1.1 referrer API requests upstream. |  | Enum: [true false] <br />Optional: \{\} <br /> |
| `proxy_cache_filter_pattern` _string_ | ProxyCacheFilterPattern limits which upstream repositories are proxied.<br />Set an explicit empty value to allow all repositories. |  | Optional: \{\} <br /> |
| `proxy_cache_filter_kind` _string_ | ProxyCacheFilterKind selects how ProxyCacheFilterPattern is matched. |  | Enum: [doublestar regex] <br />Optional: \{\} <br /> |


#### ProjectReference



ProjectReference identifies a Project custom resource.



_Appears in:_
- [ImmutableTagRuleSpec](#immutabletagrulespec)
- [LabelSpec](#labelspec)
- [MemberSpec](#memberspec)
- [PreheatPolicySpec](#preheatpolicyspec)
- [QuotaSpec](#quotaspec)
- [RetentionPolicySpec](#retentionpolicyspec)
- [RobotPermission](#robotpermission)
- [WebhookPolicySpec](#webhookpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the Project resource. |  | MaxLength: 253 <br />MinLength: 1 <br />Pattern: `^[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?(\.[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?)*$` <br /> |
| `namespace` _string_ | Namespace of the Project resource. Defaults to the referencing resource namespace. |  | MaxLength: 63 <br />Pattern: `^[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?$` <br />Optional: \{\} <br /> |


#### ProjectSpec



ProjectSpec defines the desired state of Project.



_Appears in:_
- [Project](#project)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls what happens when the Kubernetes object is deleted.<br />Delete removes the corresponding Harbor resource before removing the finalizer.<br />Orphan skips Harbor-side deletion and removes the finalizer so the<br />Kubernetes object can be deleted while leaving the Harbor resource in place.<br />Defaults to Delete. | Delete | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift.<br />When omitted, the operator's default drift detection interval is used.<br />An explicit value of 0 disables periodic drift detection. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `creationPolicy` _[CreationPolicy](#creationpolicy)_ | CreationPolicy controls whether the operator creates or adopts the Harbor project.<br />When omitted, the operator's default creation policy is used. |  | Enum: [Create Adopt CreateOrAdopt] <br />Optional: \{\} <br /> |
| `public` _boolean_ | Public indicates whether the project is public. |  |  |
| `owner` _string_ | Owner is an optional field for the project owner. |  | Optional: \{\} <br /> |
| `metadata` _[ProjectMetadata](#projectmetadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `cveAllowlist` _[CVEAllowlist](#cveallowlist)_ | CVEAllowlist configures the project's CVE allowlist. Omit it to leave the<br />existing allowlist unmanaged; provide an empty items list to clear it. |  | Optional: \{\} <br /> |
| `registryRef` _[RegistryReference](#registryreference)_ | RegistryRef references the Registry to use for proxy cache projects. |  | Optional: \{\} <br /> |


#### PurgeAuditParameters



PurgeAuditParameters defines parameters for purge audit schedules.



_Appears in:_
- [PurgeAuditScheduleSpec](#purgeauditschedulespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `auditRetentionHour` _integer_ | AuditRetentionHour is the retention period in hours. |  | Optional: \{\} <br /> |
| `includeEventTypes` _string_ | IncludeEventTypes is a comma-separated list of event types to include. |  | Optional: \{\} <br /> |
| `dryRun` _boolean_ | DryRun indicates whether to run in dry-run mode. Defaults to false. | false | Optional: \{\} <br /> |


#### PurgeAuditSchedule



PurgeAuditSchedule is the Schema for the purgeauditschedules API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `PurgeAuditSchedule` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[PurgeAuditScheduleSpec](#purgeauditschedulespec)_ |  |  |  |


#### PurgeAuditScheduleSpec



PurgeAuditScheduleSpec defines the desired schedule for audit purge.



_Appears in:_
- [PurgeAuditSchedule](#purgeauditschedule)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls what happens when the Kubernetes object is deleted.<br />Delete removes the corresponding Harbor resource before removing the finalizer.<br />Orphan skips Harbor-side deletion and removes the finalizer so the<br />Kubernetes object can be deleted while leaving the Harbor resource in place.<br />Defaults to Delete. | Delete | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift.<br />When omitted, the operator's default drift detection interval is used.<br />An explicit value of 0 disables periodic drift detection. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `schedule` _[ScheduleSpec](#schedulespec)_ | Schedule defines when purge runs. |  |  |
| `parameters` _[PurgeAuditParameters](#purgeauditparameters)_ | Parameters define purge settings. |  | Optional: \{\} <br /> |


#### Quota



Quota is the Schema for the quotas API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `Quota` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[QuotaSpec](#quotaspec)_ |  |  |  |


#### QuotaSpec



QuotaSpec defines the desired state of Quota.



_Appears in:_
- [Quota](#quota)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls what happens when the Kubernetes object is deleted.<br />Delete removes the corresponding Harbor resource before removing the finalizer.<br />Orphan skips Harbor-side deletion and removes the finalizer so the<br />Kubernetes object can be deleted while leaving the Harbor resource in place.<br />Defaults to Delete. | Delete | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift.<br />When omitted, the operator's default drift detection interval is used.<br />An explicit value of 0 disables periodic drift detection. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `projectRef` _[ProjectReference](#projectreference)_ | ProjectRef references a Project CR to derive the Harbor project ID. |  | Optional: \{\} <br /> |
| `hard` _object (keys:string, values:integer)_ | Hard defines the quota hard limits (resource name -> limit). |  | Optional: \{\} <br /> |


#### Registry



Registry is the Schema for the registries API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `Registry` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RegistrySpec](#registryspec)_ |  |  |  |


#### RegistryCredentialSpec



RegistryCredentialSpec defines registry authentication details.



_Appears in:_
- [RegistrySpec](#registryspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _string_ | Type of the credential, e.g. "basic" or "oauth". |  | Enum: [basic oauth] <br /> |
| `accessKeySecretRef` _[SecretReference](#secretreference)_ | AccessKeySecretRef references the secret key holding the access key (username). |  |  |
| `accessSecretSecretRef` _[SecretReference](#secretreference)_ | AccessSecretSecretRef references the secret key holding the access secret (password/token). |  |  |


#### RegistryReference



RegistryReference identifies a Registry custom resource.



_Appears in:_
- [ProjectSpec](#projectspec)
- [ReplicationPolicySpec](#replicationpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the Registry resource. |  | MaxLength: 253 <br />MinLength: 1 <br />Pattern: `^[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?(\.[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?)*$` <br /> |
| `namespace` _string_ | Namespace of the Registry resource. Defaults to the referencing resource namespace. |  | MaxLength: 63 <br />Pattern: `^[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?$` <br />Optional: \{\} <br /> |


#### RegistrySpec



RegistrySpec defines the desired state of Registry.



_Appears in:_
- [Registry](#registry)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls what happens when the Kubernetes object is deleted.<br />Delete removes the corresponding Harbor resource before removing the finalizer.<br />Orphan skips Harbor-side deletion and removes the finalizer so the<br />Kubernetes object can be deleted while leaving the Harbor resource in place.<br />Defaults to Delete. | Delete | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift.<br />When omitted, the operator's default drift detection interval is used.<br />An explicit value of 0 disables periodic drift detection. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `creationPolicy` _[CreationPolicy](#creationpolicy)_ | CreationPolicy controls whether the operator creates or adopts the Harbor registry.<br />When omitted, the operator's default creation policy is used. |  | Enum: [Create Adopt CreateOrAdopt] <br />Optional: \{\} <br /> |
| `type` _string_ | Type of the registry, e.g., "github-ghcr". |  | Enum: [github-ghcr ali-acr aws-ecr azure-acr docker-hub docker-registry google-gcr harbor huawei-SWR jfrog-artifactory tencent-tcr volcengine-cr] <br /> |
| `description` _string_ | Description is an optional description. |  | Optional: \{\} <br /> |
| `url` _string_ | URL is the absolute HTTP or HTTPS URL of the registry endpoint. |  | Format: uri <br />Pattern: `^https?://[^/?#]+(/[^?#]*)?$` <br /> |
| `credential` _[RegistryCredentialSpec](#registrycredentialspec)_ | Credential holds authentication details for the registry. |  | Optional: \{\} <br /> |
| `caCertificate` _string_ | CACertificate is the PEM-encoded CA certificate for this registry endpoint. |  | Optional: \{\} <br /> |
| `caCertificateRef` _[SecretReference](#secretreference)_ | CACertificateRef references a secret value holding the PEM-encoded CA certificate.<br />It is mutually exclusive with caCertificate. |  | Optional: \{\} <br /> |
| `insecure` _boolean_ | Insecure disables TLS certificate verification when Harbor connects to the registry.<br />Defaults to false. Enabling it is insecure; prefer CACertificateRef for private CAs. | false | Optional: \{\} <br /> |


#### ReplicationFilterSpec



ReplicationFilterSpec defines a replication filter.



_Appears in:_
- [ReplicationPolicySpec](#replicationpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _string_ | Type defines the filter type. |  | Optional: \{\} <br /> |
| `value` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#json-v1-apiextensions-k8s-io)_ | Value defines the filter value. |  | Optional: \{\} <br /> |
| `decoration` _string_ | Decoration defines how to interpret the filter. |  | Optional: \{\} <br /> |


#### ReplicationPolicy



ReplicationPolicy is the Schema for the replicationpolicies API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `ReplicationPolicy` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ReplicationPolicySpec](#replicationpolicyspec)_ |  |  |  |


#### ReplicationPolicySpec



ReplicationPolicySpec defines the desired state of ReplicationPolicy.



_Appears in:_
- [ReplicationPolicy](#replicationpolicy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls what happens when the Kubernetes object is deleted.<br />Delete removes the corresponding Harbor resource before removing the finalizer.<br />Orphan skips Harbor-side deletion and removes the finalizer so the<br />Kubernetes object can be deleted while leaving the Harbor resource in place.<br />Defaults to Delete. | Delete | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift.<br />When omitted, the operator's default drift detection interval is used.<br />An explicit value of 0 disables periodic drift detection. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `creationPolicy` _[CreationPolicy](#creationpolicy)_ | CreationPolicy controls whether the operator creates or adopts the Harbor policy.<br />When omitted, the operator's default creation policy is used. |  | Enum: [Create Adopt CreateOrAdopt] <br />Optional: \{\} <br /> |
| `description` _string_ | Description is an optional policy description. |  | Optional: \{\} <br /> |
| `sourceRegistryRef` _[RegistryReference](#registryreference)_ | SourceRegistryRef references a Registry CR to use as the source. |  | Optional: \{\} <br /> |
| `destinationRegistryRef` _[RegistryReference](#registryreference)_ | DestinationRegistryRef references a Registry CR to use as the destination. |  | Optional: \{\} <br /> |
| `destNamespace` _string_ | DestNamespace is the destination namespace. |  | Optional: \{\} <br /> |
| `destNamespaceReplaceCount` _integer_ | DestNamespaceReplaceCount controls how many path components are replaced by DestNamespace.<br />Defaults to -1, which selects Harbor's legacy replacement behavior. | -1 | Optional: \{\} <br /> |
| `trigger` _[ReplicationTriggerSpec](#replicationtriggerspec)_ | Trigger defines when the replication policy runs. |  | Optional: \{\} <br /> |
| `filters` _[ReplicationFilterSpec](#replicationfilterspec) array_ | Filters defines the replication filters. |  | Optional: \{\} <br /> |
| `replicateDeletion` _boolean_ | ReplicateDeletion indicates whether delete operations are replicated.<br />When omitted, the operator leaves this field unset for Harbor to interpret. |  | Optional: \{\} <br /> |
| `override` _boolean_ | Override indicates whether to overwrite destination resources.<br />When omitted, the operator leaves this field unset for Harbor to interpret. |  | Optional: \{\} <br /> |
| `enabled` _boolean_ | Enabled indicates whether the policy is enabled.<br />When omitted, the operator leaves this field unset for Harbor to interpret. |  | Optional: \{\} <br /> |
| `speed` _integer_ | Speed is the speed limit for each task.<br />When omitted, the operator leaves this field unset for Harbor to interpret. |  | Optional: \{\} <br /> |
| `copyByChunk` _boolean_ | CopyByChunk indicates whether to enable copy by chunk.<br />When omitted, the operator leaves this field unset for Harbor to interpret. |  | Optional: \{\} <br /> |
| `singleActiveReplication` _boolean_ | SingleActiveReplication avoids overlapping executions.<br />When omitted, the operator leaves this field unset for Harbor to interpret. |  | Optional: \{\} <br /> |


#### ReplicationTriggerSettings



ReplicationTriggerSettings defines settings for a replication trigger.



_Appears in:_
- [ReplicationTriggerSpec](#replicationtriggerspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cron` _string_ | Cron is the cron expression for scheduled triggers. |  | MinLength: 1 <br />Optional: \{\} <br /> |


#### ReplicationTriggerSpec



ReplicationTriggerSpec defines when the replication policy runs.



_Appears in:_
- [ReplicationPolicySpec](#replicationpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _string_ | Type defines the trigger type (manual, event_based, scheduled). |  | Enum: [manual event_based scheduled] <br />Optional: \{\} <br /> |
| `settings` _[ReplicationTriggerSettings](#replicationtriggersettings)_ | Settings holds trigger settings. |  | Optional: \{\} <br /> |


#### RetentionPolicy



RetentionPolicy is the Schema for the retentionpolicies API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `RetentionPolicy` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RetentionPolicySpec](#retentionpolicyspec)_ |  |  |  |


#### RetentionPolicySpec



RetentionPolicySpec defines the desired state of a retention policy.



_Appears in:_
- [RetentionPolicy](#retentionpolicy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls what happens when the Kubernetes object is deleted.<br />Delete removes the corresponding Harbor resource before removing the finalizer.<br />Orphan skips Harbor-side deletion and removes the finalizer so the<br />Kubernetes object can be deleted while leaving the Harbor resource in place.<br />Defaults to Delete. | Delete | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift.<br />When omitted, the operator's default drift detection interval is used.<br />An explicit value of 0 disables periodic drift detection. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `projectRef` _[ProjectReference](#projectreference)_ | ProjectRef references a Project CR to derive the Harbor project ID.<br />When set, scope.ref is resolved from the Project status and scope.level is forced to "project". |  | Optional: \{\} <br /> |
| `rules` _[RetentionRule](#retentionrule) array_ | Rules defines the retention rules. |  | MinItems: 1 <br /> |
| `trigger` _[RetentionTrigger](#retentiontrigger)_ | Trigger defines when the retention policy runs. |  | Optional: \{\} <br /> |
| `scope` _[RetentionScope](#retentionscope)_ | Scope defines the policy scope. |  | Optional: \{\} <br /> |


#### RetentionRule



RetentionRule defines a retention rule.



_Appears in:_
- [RetentionPolicySpec](#retentionpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `disabled` _boolean_ | Disabled indicates whether the rule is disabled. Defaults to false. | false | Optional: \{\} <br /> |
| `action` _string_ | Action defines the rule action, e.g. "delete". |  | Optional: \{\} <br /> |
| `template` _string_ | Template defines the rule template. |  | Optional: \{\} <br /> |
| `params` _object (keys:string, values:[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#json-v1-apiextensions-k8s-io))_ | Params holds template parameters. |  | Optional: \{\} <br /> |
| `tagSelectors` _[RetentionSelector](#retentionselector) array_ | TagSelectors define the tag selectors. |  | Optional: \{\} <br /> |
| `scopeSelectors` _object (keys:string, values:[RetentionSelector](#retentionselector))_ | ScopeSelectors define the scope selectors. |  | Optional: \{\} <br /> |


#### RetentionScope



RetentionScope defines policy scope.



_Appears in:_
- [RetentionPolicySpec](#retentionpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `level` _string_ | Level defines scope level, e.g. "project". |  | Optional: \{\} <br /> |
| `ref` _integer_ | Ref is the scope reference. |  | Optional: \{\} <br /> |


#### RetentionSelector



RetentionSelector defines a selector.



_Appears in:_
- [RetentionRule](#retentionrule)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind defines selector kind. |  | Optional: \{\} <br /> |
| `decoration` _string_ | Decoration defines selector decoration. |  | Optional: \{\} <br /> |
| `pattern` _string_ | Pattern defines selector pattern. |  | Optional: \{\} <br /> |
| `extras` _string_ | Extras defines extra selector details. |  | Optional: \{\} <br /> |


#### RetentionTrigger



RetentionTrigger defines when a policy runs.



_Appears in:_
- [RetentionPolicySpec](#retentionpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind defines trigger kind. |  | Optional: \{\} <br /> |
| `settings` _object (keys:string, values:[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#json-v1-apiextensions-k8s-io))_ | Settings holds trigger settings. |  | Optional: \{\} <br /> |
| `references` _object (keys:string, values:[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#json-v1-apiextensions-k8s-io))_ | References holds trigger references. |  | Optional: \{\} <br /> |


#### Robot



Robot is the Schema for the robots API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `Robot` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RobotSpec](#robotspec)_ |  |  |  |


#### RobotAccess



RobotAccess defines a single access rule for a robot account.



_Appears in:_
- [RobotPermission](#robotpermission)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resource` _[RobotResource](#robotresource)_ | Resource defines the resource to grant access to. |  | Enum: [* configuration label log ldap-user member metadata quota repository tag-retention immutable-tag robot notification-policy scan sbom scanner artifact tag accessory artifact-addition artifact-label preheat-policy preheat-instance audit-log catalog project user user-group registry replication distribution garbage-collection replication-adapter replication-policy scan-all system-volumes purge-audit export-cve jobservice-monitor security-hub] <br /> |
| `action` _[RobotAction](#robotaction)_ | Action defines the action to permit. |  | Enum: [* pull push create read update delete list operate scanner-pull stop] <br /> |
| `effect` _string_ | Effect defines the effect of the access rule. Defaults to allow. | allow | Optional: \{\} <br /> |


#### RobotAction

_Underlying type:_ _string_

RobotAction is the action of a robot permission access rule.



_Appears in:_
- [RobotAccess](#robotaccess)

| Field | Description |
| --- | --- |
| `*` |  |
| `pull` |  |
| `push` |  |
| `create` |  |
| `read` |  |
| `update` |  |
| `delete` |  |
| `list` |  |
| `operate` |  |
| `scanner-pull` |  |
| `stop` |  |


#### RobotPermission



RobotPermission defines a permission block for a robot account.



_Appears in:_
- [RobotSpec](#robotspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind defines the permission scope, such as "project" or "system". |  | Enum: [project system] <br /> |
| `projectRef` _[ProjectReference](#projectreference)_ | ProjectRef references the Project CR for project-scoped permissions.<br />When omitted for kind=project, Harbor's native all-projects scope is used. |  | Optional: \{\} <br /> |
| `access` _[RobotAccess](#robotaccess) array_ | Access lists the access rules for this permission. |  | MinItems: 1 <br /> |


#### RobotResource

_Underlying type:_ _string_

RobotResource is the resource of a robot permission access rule.



_Appears in:_
- [RobotAccess](#robotaccess)

| Field | Description |
| --- | --- |
| `*` |  |
| `configuration` |  |
| `label` |  |
| `log` |  |
| `ldap-user` |  |
| `member` |  |
| `metadata` |  |
| `quota` |  |
| `repository` |  |
| `tag-retention` |  |
| `immutable-tag` |  |
| `robot` |  |
| `notification-policy` |  |
| `scan` |  |
| `sbom` |  |
| `scanner` |  |
| `artifact` |  |
| `tag` |  |
| `accessory` |  |
| `artifact-addition` |  |
| `artifact-label` |  |
| `preheat-policy` |  |
| `preheat-instance` |  |
| `audit-log` |  |
| `catalog` |  |
| `project` |  |
| `user` |  |
| `user-group` |  |
| `registry` |  |
| `replication` |  |
| `distribution` |  |
| `garbage-collection` |  |
| `replication-adapter` |  |
| `replication-policy` |  |
| `scan-all` |  |
| `system-volumes` |  |
| `purge-audit` |  |
| `export-cve` |  |
| `jobservice-monitor` |  |
| `security-hub` |  |


#### RobotSpec



RobotSpec defines the desired state of Robot.



_Appears in:_
- [Robot](#robot)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls what happens when the Kubernetes object is deleted.<br />Delete removes the corresponding Harbor resource before removing the finalizer.<br />Orphan skips Harbor-side deletion and removes the finalizer so the<br />Kubernetes object can be deleted while leaving the Harbor resource in place.<br />Defaults to Delete. | Delete | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift.<br />When omitted, the operator's default drift detection interval is used.<br />An explicit value of 0 disables periodic drift detection. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `creationPolicy` _[CreationPolicy](#creationpolicy)_ | CreationPolicy controls whether the operator creates or adopts the Harbor robot.<br />When omitted, the operator's default creation policy is used. |  | Enum: [Create Adopt CreateOrAdopt] <br />Optional: \{\} <br /> |
| `description` _string_ | Description of the robot account. |  | Optional: \{\} <br /> |
| `level` _string_ | Level is the scope of the robot account.<br />Allowed values: "system", "project". |  | Enum: [system project] <br /> |
| `permissions` _[RobotPermission](#robotpermission) array_ | Permissions define the access granted to the robot account. |  | MinItems: 1 <br /> |
| `disable` _boolean_ | Disable controls whether the robot account is disabled.<br />When omitted, the operator leaves it unset on creation and preserves the current value on update. |  | Optional: \{\} <br /> |
| `duration` _integer_ | Duration is the token duration in days. Use -1 for never expires.<br />If omitted, it defaults to -1. | -1 | Optional: \{\} <br /> |
| `secretRef` _[SecretReference](#secretreference)_ | SecretRef references the operator-managed secret key holding the robot secret.<br />The operator writes the generated robot secret to this location and expects<br />the Secret to either not exist yet or already be managed by this Robot.<br />The Secret also contains the canonical Harbor robot username under the<br />reserved key "username".<br />At least one of secretRef or imagePullSecretRef must be set. |  | Optional: \{\} <br /> |
| `imagePullSecretRef` _[SecretObjectReference](#secretobjectreference)_ | ImagePullSecretRef references the operator-managed Secret that contains<br />a Kubernetes dockerconfigjson image-pull credential for this robot.<br />The Secret is written with the Harbor registry host from the resolved<br />Harbor connection and the current robot credentials. Its namespace<br />defaults to the Robot namespace. |  | Optional: \{\} <br /> |


#### ScanAllSchedule



ScanAllSchedule is the Schema for the scanallschedules API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `ScanAllSchedule` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ScanAllScheduleSpec](#scanallschedulespec)_ |  |  |  |


#### ScanAllScheduleSpec



ScanAllScheduleSpec defines the desired schedule for scan all.



_Appears in:_
- [ScanAllSchedule](#scanallschedule)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls what happens when the Kubernetes object is deleted.<br />Delete removes the corresponding Harbor resource before removing the finalizer.<br />Orphan skips Harbor-side deletion and removes the finalizer so the<br />Kubernetes object can be deleted while leaving the Harbor resource in place.<br />Defaults to Delete. | Delete | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift.<br />When omitted, the operator's default drift detection interval is used.<br />An explicit value of 0 disables periodic drift detection. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `schedule` _[ScheduleSpec](#schedulespec)_ | Schedule defines when scan all runs. |  |  |
| `parameters` _object (keys:string, values:[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#json-v1-apiextensions-k8s-io))_ | Parameters define scan all settings passed to Harbor. |  | Optional: \{\} <br /> |


#### ScannerRegistration



ScannerRegistration is the Schema for the scannerregistrations API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `ScannerRegistration` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ScannerRegistrationSpec](#scannerregistrationspec)_ |  |  |  |


#### ScannerRegistrationSpec



ScannerRegistrationSpec defines the desired state of ScannerRegistration.



_Appears in:_
- [ScannerRegistration](#scannerregistration)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls what happens when the Kubernetes object is deleted.<br />Delete removes the corresponding Harbor resource before removing the finalizer.<br />Orphan skips Harbor-side deletion and removes the finalizer so the<br />Kubernetes object can be deleted while leaving the Harbor resource in place.<br />Defaults to Delete. | Delete | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift.<br />When omitted, the operator's default drift detection interval is used.<br />An explicit value of 0 disables periodic drift detection. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `creationPolicy` _[CreationPolicy](#creationpolicy)_ | CreationPolicy controls whether the operator creates or adopts the Harbor scanner registration.<br />When omitted, the operator's default creation policy is used. |  | Enum: [Create Adopt CreateOrAdopt] <br />Optional: \{\} <br /> |
| `description` _string_ | Description is an optional description. |  | Optional: \{\} <br /> |
| `url` _string_ | URL is the scanner adapter base URL. |  | Format: uri <br /> |
| `auth` _string_ | Auth defines the authentication approach (e.g. Basic, Bearer, X-ScannerAdapter-API-Key). |  | Optional: \{\} <br /> |
| `accessCredential` _string_ | AccessCredential is the credential value sent in the auth header. |  | Optional: \{\} <br /> |
| `accessCredentialSecretRef` _[SecretReference](#secretreference)_ | AccessCredentialSecretRef references a secret value holding the credential. |  | Optional: \{\} <br /> |
| `skipCertVerify` _boolean_ | SkipCertVerify disables TLS certificate verification for scanner requests.<br />Defaults to false. Enabling it is insecure. | false | Optional: \{\} <br /> |
| `useInternalAddr` _boolean_ | UseInternalAddr indicates whether the scanner uses Harbor's internal address.<br />Defaults to false. | false | Optional: \{\} <br /> |
| `disabled` _boolean_ | Disabled indicates whether the registration is disabled. Defaults to false. | false | Optional: \{\} <br /> |
| `default` _boolean_ | Default promotes this scanner to the system default when true.<br />False or omitted does not change Harbor's current default scanner assignment. |  | Optional: \{\} <br /> |


#### ScheduleSpec



ScheduleSpec defines the schedule configuration.



_Appears in:_
- [GCScheduleSpec](#gcschedulespec)
- [PurgeAuditScheduleSpec](#purgeauditschedulespec)
- [ScanAllScheduleSpec](#scanallschedulespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _string_ | Type defines the schedule type.<br />Valid values: Hourly, Daily, Weekly, Custom, Manual, None, Schedule. |  | Enum: [Hourly Daily Weekly Custom Manual None Schedule] <br /> |
| `cron` _string_ | Cron is the cron expression when Type is not Manual or None. |  | Optional: \{\} <br /> |


#### SecretObjectReference



SecretObjectReference identifies a Secret object without selecting a key.
It is used for operator-managed Secret outputs whose data shape is fixed by
the referencing resource.



_Appears in:_
- [PreheatInstanceSpec](#preheatinstancespec)
- [RobotSpec](#robotspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the Secret. |  | MaxLength: 253 <br />MinLength: 1 <br />Pattern: `^[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?(\.[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?)*$` <br /> |
| `namespace` _string_ | Namespace of the Secret. When omitted, the controller uses the namespace<br />of the referencing namespaced resource. |  | MaxLength: 63 <br />Pattern: `^[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?$` <br />Optional: \{\} <br /> |


#### SecretReference



SecretReference is similar to a corev1.SecretKeySelector but allows
cross-namespace references when enabled in the operator RBAC.



_Appears in:_
- [ConfigurationValueSource](#configurationvaluesource)
- [Credentials](#credentials)
- [HarborConnectionSpec](#harborconnectionspec)
- [RegistryCredentialSpec](#registrycredentialspec)
- [RegistrySpec](#registryspec)
- [RobotSpec](#robotspec)
- [ScannerRegistrationSpec](#scannerregistrationspec)
- [UserSpec](#userspec)
- [WebhookTargetSpec](#webhooktargetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the Secret. |  | MaxLength: 253 <br />MinLength: 1 <br />Pattern: `^[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?(\.[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?)*$` <br /> |
| `key` _string_ | Key inside the Secret data. When omitted, the controller using this<br />reference will apply a sensible default. |  | Optional: \{\} <br /> |
| `namespace` _string_ | Namespace of the Secret. When omitted, the controller uses the namespace of<br />the referencing namespaced resource. References from cluster-scoped resources<br />must set this field explicitly because they have no namespace. |  | MaxLength: 63 <br />Pattern: `^[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?$` <br />Optional: \{\} <br /> |


#### User



User is the Schema for the users API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `User` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[UserSpec](#userspec)_ |  |  |  |


#### UserGroupClaim



UserGroupClaim is the Schema for the usergroupclaims API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `UserGroupClaim` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[UserGroupClaimSpec](#usergroupclaimspec)_ |  |  |  |


#### UserGroupClaimReference



UserGroupClaimReference identifies a UserGroupClaim custom resource.



_Appears in:_
- [MemberGroup](#membergroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the UserGroupClaim resource. |  | MaxLength: 253 <br />MinLength: 1 <br />Pattern: `^[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?(\.[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?)*$` <br /> |
| `namespace` _string_ | Namespace of the UserGroupClaim resource. Defaults to the referencing resource namespace. |  | MaxLength: 63 <br />Pattern: `^[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?$` <br />Optional: \{\} <br /> |


#### UserGroupClaimSpec



UserGroupClaimSpec describes an external identity that must be registered
in Harbor. The claim is intentionally non-owning: deleting it never deletes
the global Harbor UserGroup because that would remove memberships belonging
to other claims.



_Appears in:_
- [UserGroupClaim](#usergroupclaim)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `groupName` _string_ | GroupName is the exact external group name stored in Harbor. For OIDC<br />groups, this is commonly the identity provider's group ID. |  | MinLength: 1 <br /> |
| `groupType` _integer_ | GroupType is the group type (1=LDAP, 2=HTTP, 3=OIDC). |  | Enum: [1 2 3] <br /> |
| `ldapGroupDN` _string_ | LDAPGroupDN is the DN of the LDAP group when GroupType is LDAP. |  | Optional: \{\} <br /> |


#### UserReference



UserReference identifies a User custom resource.



_Appears in:_
- [MemberUser](#memberuser)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the User resource. |  | MaxLength: 253 <br />MinLength: 1 <br />Pattern: `^[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?(\.[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?)*$` <br /> |
| `namespace` _string_ | Namespace of the User resource. Defaults to the referencing resource namespace. |  | MaxLength: 63 <br />Pattern: `^[a-z0-9]([-a-z0-9]\{0,61\}[a-z0-9])?$` <br />Optional: \{\} <br /> |


#### UserSpec



UserSpec defines the desired state of User.



_Appears in:_
- [User](#user)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls what happens when the Kubernetes object is deleted.<br />Delete removes the corresponding Harbor resource before removing the finalizer.<br />Orphan skips Harbor-side deletion and removes the finalizer so the<br />Kubernetes object can be deleted while leaving the Harbor resource in place.<br />Defaults to Delete. | Delete | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift.<br />When omitted, the operator's default drift detection interval is used.<br />An explicit value of 0 disables periodic drift detection. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `creationPolicy` _[CreationPolicy](#creationpolicy)_ | CreationPolicy controls whether the operator creates or adopts the Harbor user.<br />When omitted, the operator's default creation policy is used. |  | Enum: [Create Adopt CreateOrAdopt] <br />Optional: \{\} <br /> |
| `email` _string_ | Email address of the user. |  | Format: email <br /> |
| `realname` _string_ | Realname is an optional full name. Defaults to metadata.name. |  | Optional: \{\} <br /> |
| `comment` _string_ | Comment is an optional comment for the user. |  | Optional: \{\} <br /> |
| `passwordSecretRef` _[SecretReference](#secretreference)_ | PasswordSecretRef references the Secret and required key containing the user's password. |  |  |


#### WebhookPolicy



WebhookPolicy is the Schema for the webhookpolicies API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `harbor.harbor-entity-operator.io/v1alpha1` | | |
| `kind` _string_ | `WebhookPolicy` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[WebhookPolicySpec](#webhookpolicyspec)_ |  |  |  |


#### WebhookPolicySpec



WebhookPolicySpec defines the desired state of WebhookPolicy.



_Appears in:_
- [WebhookPolicy](#webhookpolicy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `harborConnectionRef` _[HarborConnectionReference](#harborconnectionreference)_ | HarborConnectionRef references the Harbor connection object to use.<br />When the operator is started with --harbor-connection, this field may be omitted. |  | Optional: \{\} <br /> |
| `deletionPolicy` _[DeletionPolicy](#deletionpolicy)_ | DeletionPolicy controls what happens when the Kubernetes object is deleted.<br />Delete removes the corresponding Harbor resource before removing the finalizer.<br />Orphan skips Harbor-side deletion and removes the finalizer so the<br />Kubernetes object can be deleted while leaving the Harbor resource in place.<br />Defaults to Delete. | Delete | Enum: [Delete Orphan] <br />Optional: \{\} <br /> |
| `driftDetectionInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#duration-v1-meta)_ | DriftDetectionInterval is the interval at which the operator checks for drift.<br />When omitted, the operator's default drift detection interval is used.<br />An explicit value of 0 disables periodic drift detection. |  | Optional: \{\} <br /> |
| `reconcileNonce` _string_ | ReconcileNonce forces an immediate reconcile when updated. |  | Optional: \{\} <br /> |
| `creationPolicy` _[CreationPolicy](#creationpolicy)_ | CreationPolicy controls whether the operator creates or adopts the Harbor webhook policy.<br />When omitted, the operator's default creation policy is used. |  | Enum: [Create Adopt CreateOrAdopt] <br />Optional: \{\} <br /> |
| `projectRef` _[ProjectReference](#projectreference)_ | ProjectRef references a Project CR to derive the Harbor project ID. |  | Optional: \{\} <br /> |
| `description` _string_ | Description is an optional policy description. |  | Optional: \{\} <br /> |
| `enabled` _boolean_ | Enabled indicates whether the policy is enabled. Defaults to true. | true | Optional: \{\} <br /> |
| `eventTypes` _string array_ | EventTypes lists the webhook event types. |  | MinItems: 1 <br /> |
| `targets` _[WebhookTargetSpec](#webhooktargetspec) array_ | Targets lists the webhook targets. |  | MinItems: 1 <br /> |


#### WebhookTargetSpec



WebhookTargetSpec defines a single webhook target.



_Appears in:_
- [WebhookPolicySpec](#webhookpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _string_ | Type defines the webhook notify type.<br />Harbor validates this value against the configured notification driver. |  | MinLength: 1 <br /> |
| `address` _string_ | Address is the webhook target address.<br />Query parameters are allowed; fragments are not sent as part of HTTP requests. |  | Format: uri <br />Pattern: `^https?://[^/?#@]+([^#]*)?$` <br /> |
| `authHeader` _string_ | AuthHeader is the auth header to send to the webhook target. |  | Optional: \{\} <br /> |
| `authHeaderSecretRef` _[SecretReference](#secretreference)_ | AuthHeaderSecretRef references a secret value holding the auth header. |  | Optional: \{\} <br /> |
| `payloadFormat` _string_ | PayloadFormat is the payload format (e.g. CloudEvents). |  | Optional: \{\} <br /> |
| `skipCertVerify` _boolean_ | SkipCertVerify disables TLS certificate verification for this target.<br />Defaults to false. Enabling it is insecure. | false | Optional: \{\} <br /> |
