# Encrypt uploaded SSH keys separately from login credentials

Status: Accepted

## Context

The gateway needs SSH credentials to connect on a user's behalf. Passkeys authenticate users to the app and do not replace target-host credentials.

## Decision

Store named, user-owned SSH private keys encrypted in SQLite, following [SQLite storage](../api/008-sqlite-storage.md). Generate the application encryption key on first initialization and retain it in a separate Docker volume. Decrypt SSH keys on the server when needed and return only identifying metadata through key-management APIs.

Accept only unencrypted Ed25519 private keys in OpenSSH private-key format (`BEGIN OPENSSH PRIVATE KEY`). Limit each uploaded key file to 16 KiB (16,384 bytes), including surrounding whitespace. Require exactly one private key, allowing surrounding whitespace but no additional content. Validate with the SSH library and enforce the format and algorithm restrictions even if the library supports more types. Reject passphrase-protected keys with a clear unsupported-passphrase message; reject malformed, oversized, and unsupported keys with clear validation errors.

Use `golang.org/x/crypto/ssh` for private-key parsing and SHA-256 public fingerprints. Keep parser details out of user-facing errors and return only the fingerprint from validation. Enforce the file-size limit before trimming whitespace; HTTP upload handlers must also bound reads before allocating the file.

Allow reuse across saved connections and block deletion while referenced. Include one unencrypted Ed25519 demo key pair under `demo/keys/` as `demo_ed25519` and `demo_ed25519.pub`, with a README. Users upload the private key; the bastion and targets authorize the matching public key when the lab hosts are introduced. Label the private key as intentionally public and only for the disposable local lab. This demo SSH identity is separate from application encryption material, which must never be committed.

## Consequences

A database dump alone does not expose usable SSH private keys. Access to both volumes defeats that protection, and the running gateway must access plaintext keys to authenticate. Losing encryption material requires re-uploading keys; the app must report the problem rather than silently replacing encryption material for existing records.
