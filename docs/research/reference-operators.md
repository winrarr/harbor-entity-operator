# Reference implementation research

Research checked on 2026-10-13.

## Sources

- [Operator Foundry](https://github.com/winrarr/operator-foundry) provides the repository structure, controller-runtime conventions, generated delivery assets, documentation workflow, and local Kind patterns used as the engineering foundation.
- [Harbor Operator](https://github.com/winrarr/harbor-operator) provides concrete Harbor resource and reconciliation examples. Its implementation is evidence for Harbor-specific lifecycle behavior, not a substitute for Harbor's API contract.
- [Harbor API v2 OpenAPI](https://github.com/goharbor/harbor/blob/35dd9e4337ca6857deffab19d28cc43df596f328/api/v2.0/swagger.yaml) is the primary source for endpoint and payload semantics.

## Applied patterns

- Keep the Harbor client typed and separate from Kubernetes reconciliation.
- Represent external lifecycles with status, finalizers, explicit adoption and deletion policies, and drift detection.
- Model project membership as a separate relationship resource so one Kubernetes object owns one membership edge.
- Keep Harbor-global identity and singleton APIs explicit in the resource scope and connection model.
- Verify generated CRDs, RBAC, Helm assets, and API reference output from source types and markers.
- Use Kind with Harbor installed for live packaging and lifecycle checks.

The external API specification remains authoritative whenever local implementation examples differ from Harbor's documented contract.
