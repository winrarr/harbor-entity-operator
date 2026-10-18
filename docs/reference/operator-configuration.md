# Operator Configuration

The Helm chart exposes the operator's runtime flags as values. These settings
control reconciliation scope and defaults; they do not replace Kubernetes
authorization or admission policies.

## Harbor connection

`harborConnection` configures one operator-wide `ClusterHarborConnection`.
When it is set, resources may omit `spec.harborConnectionRef` and use this
connection instead. If a resource includes `spec.harborConnectionRef`, it must
reference that same `ClusterHarborConnection`; a different reference is
rejected.

## Reconciliation scope

`watchNamespaces` limits the namespaces cached and reconciled by the operator.
It is an operational scope and is not a tenant-isolation boundary. Use
namespace RBAC and admission policies, or set
`allowCrossNamespaceReferences: false`, when you need tenant boundaries.

Use `watchNamespaces` when one operator deployment should manage only a fixed
subset of namespaces, reduce its operational blast radius, or separate
operator instances by workload. Leaving it empty watches all namespaces.

With `allowCrossNamespaceReferences: false`, a resource may reference only
objects in its own namespace. The default is `true`, so cross-namespace
references are allowed unless disabled.

## Creation and drift defaults

`defaultCreationPolicy` supplies the creation policy when a resource does not
specify one. The supported policies are `Create`, `Adopt`, and `CreateOrAdopt`.
An explicit `spec.creationPolicy` always wins over the operator default.

`defaultDriftDetectionInterval` controls how often managed resources are
checked for drift when they are otherwise idle. Set it to `0s` to disable the
periodic check if that is appropriate for your installation.

## Harbor API traffic

`harborRequestTimeout` bounds the duration of each Harbor API request and
defaults to `30s`. `harborMaxConcurrentRequests` caps in-flight Harbor API
requests across every controller in one operator process and defaults to `20`.
Set the limit to a positive integer when the Helm chart is installed. This
limits simultaneous slow requests; it does not impose a requests-per-second
limit. The operator currently has no evidence that an additional rate limiter
is needed, so request rate remains observable through the existing Harbor API
request metrics. The Helm values schema rejects zero and negative limits.

Harbor HTTP clients are cached per resolved connection, up to 64 connections.
Changing the endpoint, credentials, CA bundle, request timeout, or concurrency
setting replaces that connection's cached client and closes its idle transport
connections. Credentials and CA data are included only in a one-way cache
fingerprint and are never used as cache labels. The
`harbor_operator_harbor_http_client_cache_events_total` metric reports cache
hits, misses, invalidations, and evictions.

## Metrics

Metrics are disabled by default. When enabled, the chart can create a
`ServiceMonitor`. Secure metrics use the chart's configured certificate; for
production, configure a certificate Secret trusted by your Prometheus
installation instead of relying on the development certificate. Harbor API
client cache events are exposed on the same metrics endpoint.

Network policy is intentionally managed outside the chart. See
[Multi-Tenancy](multi-tenancy.md#network-policy) for minimal Kubernetes and
Cilium policy examples, including the operator's API, DNS, metrics, and Harbor
traffic requirements.

See the chart's values and README for the complete list of values and their
defaults.
