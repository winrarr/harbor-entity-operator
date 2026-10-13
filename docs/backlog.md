# Backlog

This file records planned outcomes that have been accepted but are not yet implemented. Remove entries when they are delivered or no longer intended.

## Publish chart-only releases from the development branch

Allow the official chart-release workflow to publish a chart-only change from
an explicitly selected source ref such as `main`, without requiring the change
to be copied to an operator release branch. This is needed when a chart change
intentionally does not belong on any existing operator maintenance line.

Keep release metadata, chart tags, OCI provenance, required checks, and stable
operator image validation as strict as they are for release-branch chart
releases. The workflow must remain explicit and auditable; manual tag creation
is not a substitute.

Acceptance criteria:

- A maintainer can select an approved source ref and chart version for a
  chart-only release.
- The workflow validates the source ref, chart version, stable `appVersion`,
  required checks, and immutable chart tag before publishing.
- Publishing does not modify an operator release branch or require an
  unrelated operator release.
- Recovery documentation explains how to resume a failed publication without
  moving or recreating an existing tag.

## Enforce the maintenance window for old release lines

Define and enforce when an operator major/minor release line falls three lines
behind the current supported line. Routine dependency updates, patch trains,
and release automation should stop for lines outside that window, while
explicit emergency or security work remains possible with deliberate approval.

The policy must account for the project's `0.x` versioning, where minor lines
carry the compatibility boundary, and for future post-`1.0` major/minor lines.
It should make the supported and unsupported status visible in documentation,
automation output, and pull requests rather than silently ignoring a branch.

Acceptance criteria:

- The repository defines exactly how the three-line support window is counted
  across `0.x` and post-`1.0` versions.
- Automated dependency updates, patch trains, and routine release workflows
  refuse or skip unsupported lines with a clear explanation.
- Maintainers have a documented, explicit path for exceptional security or
  emergency updates.
- Tests cover the boundary between supported and unsupported release lines.

## Limit aggregate Harbor API concurrency

Add `--harbor-max-concurrent-requests` and a corresponding Helm value to cap the total number of in-flight Harbor API requests across all controllers in one operator process. The limit should protect constrained Harbor installations during request bursts without being multiplied independently per controller.

Before implementation, decide the backwards-compatible default and whether request-rate limiting is also needed; concurrency limits bound simultaneous slow requests but do not bound requests per second.

## Cache Harbor clients per connection

Reuse Harbor HTTP clients or transports for repeated reconciliations targeting the same resolved Harbor connection. The goal is to preserve connection pooling and reduce repeated transport setup without changing reconciliation behavior or implying read-your-writes consistency.

The cache must account for Harbor connection identity, endpoint, credentials, CA material, and request settings. Credential or CA changes must invalidate affected entries, and the cache must not retain unbounded entries. Only enable a read-your-writes mechanism separately if Harbor provides a supported consistency protocol and a real stale-read failure has been reproduced.

Acceptance criteria:

- Repeated reconciliations for an unchanged Harbor connection reuse HTTP connection-pooling state.
- Changes to the endpoint, credentials, CA material, or relevant request settings cause subsequent requests to use a fresh client or transport.
- Cache size and cleanup behavior are bounded and safe for multiple Harbor connections.
- Tests cover cache reuse and invalidation without exposing credentials in cache keys, logs, or metrics.
- Benchmarks or metrics demonstrate whether client or transport reuse materially reduces request setup overhead.

## Customize metadata on generated Secret outputs

Add a shared Secret-output reference shape for operator-generated Secrets. It should allow users to set the destination name, optional namespace, labels, and annotations while keeping Secret type, credential data, ownership metadata, and reserved operator metadata under controller control. Use the shape consistently for generated Secret outputs such as Robot credentials and image-pull Secrets.

The output metadata must be applied on create and reconcile, merge safely with operator-managed metadata, and reject attempts to override reserved labels or annotations. This should allow tenants to add a unique label to a Robot image-pull Secret and distribute that Secret with Capsule `TenantResource` without requiring a dedicated source namespace.

Add API validation, reconciliation tests, CRD and API documentation updates, and an example showing a custom selector label for cross-namespace Secret distribution. Preserve the existing JSON fields and behavior for users that do not configure output metadata.

## Validate project robot name length

Validate project-scoped Robot names before creation so the effective Harbor name, composed of the project name, separator, and requested robot name, fits Harbor's persisted robot-name limit. Return a clear Ready condition message instead of relying on a low-level Harbor or database error.

Do not truncate or silently rewrite Kubernetes resource names. System-scoped robots should continue to use the Kubernetes metadata name directly, and the validation should account for Harbor's configured robot username prefix only when reporting the resulting username; the persisted project-name limit is the constraint that must be enforced.

Acceptance criteria:

- Project-scoped Robot reconciliation rejects an effective name longer than Harbor permits before issuing a create request.
- The status explains the project name, requested Robot name, and applicable limit without exposing credentials.
- Boundary tests cover valid and invalid project-scoped names, including long project names, and confirm system-scoped names are unaffected.
- Robot documentation describes the constraint and the operator's behavior.
