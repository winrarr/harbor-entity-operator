# Backlog

## Project scanner assignment

Allow a Harbor `Project` to declare its selected scanner through a reference to a managed `ScannerRegistration`.

Harbor exposes project scanner selection through `GET` and `PUT /projects/{project_name_or_id}/scanner`. The operator already manages projects and scanner registrations, but does not reconcile the relationship between them.

Acceptance criteria:

- A Project can select a ScannerRegistration using a Kubernetes reference, without requiring a Harbor UUID in the spec.
- Reconciliation waits for and validates the referenced scanner, then applies and detects drift in Harbor's project scanner selection.
- Omitted and changed references have documented, tested behavior.
- The Project guide and examples describe scanner selection, and generated CRDs and API reference are current.

## Harbor user password rotation

Reconcile changes to the password Secret referenced by an existing Harbor `User`.

The User API already requires a password Secret, and Harbor exposes `PUT /users/{user_id}/password`, but the current reconciler uses the Secret only when it creates a user. Operators cannot rotate an existing managed user's password through the declared state.

Acceptance criteria:

- Updating the referenced Secret changes the password of an existing Harbor user.
- Reconciliation does not repeatedly submit an unchanged password, and retries a failed rotation.
- Password contents never appear in resource status, logs, or error messages.
- The User guide describes password rotation and its Harbor authentication-mode constraints.
- HTTP and reconciliation tests cover rotation, idempotency, and failure reporting.

## Harbor system-admin assignment

Allow an explicitly authorized Kubernetes declaration to promote or demote a Harbor user as a system administrator.

Harbor exposes `PUT /users/{user_id}/sysadmin`, and the user response reports `sysadmin_flag`; the current User API does not manage that state. System-admin status is Harbor-global privilege, so ordinary namespace-level control of a namespaced User must not silently grant that privilege.

Acceptance criteria:

- The API can express both administrator and non-administrator desired state and reconciliation detects remote drift.
- The resource boundary prevents an ordinary namespaced User writer from granting or revoking Harbor-global administrator privilege.
- The behavior is covered by Harbor client and observable reconciliation tests, and its RBAC and security constraints are documented.
