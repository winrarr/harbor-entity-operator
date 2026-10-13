# 0001. Model Harbor control-plane desired state

- Status: accepted
- Date: 2026-10-13

## Decision

Expose stable Harbor control-plane resources and singleton configuration as typed Kubernetes CRDs. Keep image contents and individual Harbor jobs outside this API. Use the official Harbor OpenAPI document as the source for endpoint and payload semantics, while designing CRDs for Kubernetes ownership, references, status, and safe lifecycle behavior.

## Rationale

Harbor's v2 API combines persistent configuration with health checks, queries, artifact operations, and job triggers. Mapping every endpoint to a CRD would make imperative actions and registry content appear to have controller-owned desired state. The operator instead models persistent resources and schedules or policies; one-time jobs and artifact contents remain owned by Harbor clients and delivery workflows.

Harbor-global identities and singleton endpoints need explicit tenancy and ownership choices. Global role, group, and provider resources use cluster connections; group deletion is blocked while it is claimed. The system CVE allowlist has one owner per Harbor instance and defaults to orphaning on deletion. Project preheat policies retain their Project and provider dependencies, and provider credentials are read from Kubernetes Secrets.

## Consequences

- Each CRD guide describes the Harbor endpoint, remote identity, references, and deletion behavior it represents.
- Harbor OpenAPI updates are reviewed for changed resource payloads and newly available desired-state APIs.
- Repository content and imperative job endpoints do not receive CRDs without a separate lifecycle design.
- Harbor's global identity, singleton, and preheat APIs have dedicated CRDs with documented ownership, dependency, credential, and deletion behavior.
