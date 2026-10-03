# Store account and passkey identities separately

Status: Accepted

## Context

Passkey login must resolve a stable account without treating a display name as
an authentication identifier. The WebAuthn library requires credential metadata
and RP scope to survive registration and subsequent logins.

## Decision

Use an application-supplied UUID as the account primary key. Display names are
nonblank labels and may repeat. Store a separate opaque WebAuthn user handle
(1–64 bytes) and RP ID on each account, unique together. Generate these stable
identifiers before registration and persist them only after verification.

Use `account_id` as the credential primary key to allow at most one passkey per
account. A composite foreign key also matches the account's RP ID. Credential
IDs are binary and unique within an RP. Account deletion cascades to its
credential. The registration transaction must persist both records together;
the schema permits an account without a credential during that transaction.

Store the public key, transports, attestation type/format, raw flags byte,
AAGUID, signature counter, clone warning, and authenticator attachment in typed
columns. Store the library's nested attestation and extension records as JSONB
objects. Retain creation and last-use timestamps. Preserve all library metadata
when mapping records; update assertion metadata after successful login. Store
flags as a small integer containing the complete protocol byte, and counters as
big integers covering the full unsigned 32-bit range.

Keep the SCS `sessions` table in its adapter-defined shape: token, encoded data,
and expiry with an expiry index. Account identity lives in server-side session
data; no account foreign key is added to the adapter-owned table. Account removal
alone does not revoke sessions; any future deletion flow must do that explicitly.

## Consequences

Discoverable login can resolve an account by RP and user handle, then confirm
credential ownership under the same RP. Names never need to be unique. One RP
per account and one credential per account match the current product scope.

The typed credential mapping must be reviewed when upgrading go-webauthn. No
private passkey material is stored. Pending single-use challenges belong to the
registration/login implementation and are not part of this migration. Application
queries and account/credential transaction handling remain with those endpoints.

This supplements [passkeys and sessions](001-passkeys-and-sessions.md) and
[auth library configuration](004-auth-libraries.md).
