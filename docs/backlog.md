# Backlog

## Project scanner assignment

Enable users to manage which vulnerability scanner Harbor uses for a project through Kubernetes.

Harbor supports project-level scanner selection, and the operator already manages projects and scanner registrations. Managing the relationship would let users keep project scanning configuration alongside the rest of their project configuration.

Acceptance criteria:

- Users can select a scanner for a Harbor project and observe that selection in Harbor.
- Changes to the desired selection are reflected in Harbor, and out-of-band changes are handled according to the documented ownership behavior.
- The behavior for an unspecified scanner and for an unavailable or invalid scanner is explicit, predictable, and documented.
- The project guide and examples explain scanner selection and its constraints.

## Harbor user password rotation

Enable users to change the password of an existing Harbor user through Kubernetes.

The operator already accepts a password Secret when creating a user, but changes to that Secret do not currently update an existing user's password. Harbor supports password changes for local users.

Acceptance criteria:

- A deliberate password change updates the credentials accepted by Harbor for an existing local user.
- Discovering or adopting an existing user does not unexpectedly replace its password.
- Unchanged desired credentials do not cause repeated password changes; failed changes are reported and can converge after the underlying problem is resolved.
- Password contents are not exposed through Kubernetes status, logs, or error messages.
- The user guide explains which Harbor authentication modes support password management and how existing users are handled.

## Harbor system-admin assignment

Enable authorized operators to manage which Harbor users have system-administrator privileges.

Harbor supports changing a user's system-administrator status, but the operator does not currently manage it. This is a Harbor-global privilege, so its management must respect the boundary between cluster-wide authority and namespace-scoped access.

Acceptance criteria:

- An appropriately authorized operator can grant and revoke system-administrator status, and Harbor reflects the declared state.
- Namespace-scoped access alone cannot grant or revoke Harbor-global administrator privileges.
- The effects of removing or deleting the declaration are explicit and predictable, including whether the Harbor privilege is retained or revoked.
- Unauthorized changes do not alter Harbor privileges and are reported clearly.
- Documentation explains the authorization boundary and lifecycle behavior.
