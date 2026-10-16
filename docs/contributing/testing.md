# Testing and Verification

## Main Test Targets

```sh
make check
make test
make test-e2e
```

- `make check` runs the normal CI baseline: generated drift, lint, tests, and the docs build
- `make test` runs deterministic Go tests and Kubernetes API contract tests with EnvTest; it does not deploy the operator or use a live Harbor instance
- `make test-e2e` runs the live suite against the current Kind cluster and Harbor instance

Keep deterministic decisions, validation, request construction, reconciliation, and error handling in the normal Go test suite. Use E2E tests only for behavior that depends on the deployed manager and its watches, real Kubernetes networking and authorization, or Harbor API semantics and remote side effects that a test server cannot reliably represent. Avoid repeating controller decisions in E2E when EnvTest or focused Go tests can verify them.

## Published artifact smoke

On 2026-10-16, the published operator image `v0.5.0` and chart `0.3.0` were installed from GHCR into an isolated Kind cluster running Kubernetes `v1.37.0`. The Harbor instance used Harbor `v2.15.4` from the upstream Harbor chart `1.19.4`.

The smoke verified a fresh OCI chart install, Harbor connection authentication, Project creation and updates, Harbor-side drift correction with a `10s` interval, Harbor credential rotation through the referenced Secret, an in-place Helm upgrade changing `harborRequestTimeout`, and Project deletion removing the remote Harbor project.

The in-place Helm upgrade reused chart `0.3.0`; the OCI registry did not have chart `0.2.0`, so this did not verify a version-to-version chart migration. This record describes tested behavior and is not a formal Harbor support matrix.

## Generated Assets

When API types, Kubebuilder markers, RBAC, or docs reference content change, regenerate and verify the generated assets:

```sh
make verify-generated
```

This preserves the generated-file diff that existed before the command and fails only when regeneration changes it further.

## Docs Site

Build the docs locally with:

```sh
make docs-build
```

Serve them locally with:

```sh
make docs-serve
```

These commands run through the pinned Zensical container image for consistency with CI.
