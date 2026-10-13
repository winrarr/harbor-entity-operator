# Resource Guides

These pages describe how each custom resource behaves in the operator.

Use the [Resource Index](../reference/resources.md) to move between a guide
and the generated API reference for exact field definitions, defaults, and
validation.

Use them for:

- example manifests
- reconciliation behavior
- Harbor-specific notes and caveats

Use the generated [API Reference](../reference/api.md) when you want the exact schema, including defaults and validation markers.

The shared `HarborSpecBase` fields are documented once in
[Common Spec Fields](../reference/common-spec-fields.md).

Identity and access resources include [HarborRole](harbor-role.md),
[HarborUserGroup](harbor-user-group.md), and the non-owning
[UserGroupClaim](usergroup.md). Instance policy resources include the singleton
[HarborCVEAllowlist](cve-allowlist.md), [PreheatInstance](preheat-instance.md),
and project-scoped [PreheatPolicy](preheat-policy.md).
