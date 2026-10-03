# Encrypt uploaded SSH keys separately from login credentials

Status: Accepted

## Context

The gateway needs SSH credentials to connect on a user's behalf. Passkeys authenticate users to the app and do not replace target-host credentials.

## Decision

Store named, user-owned SSH private keys encrypted in Postgres. Generate the application encryption key on first initialization and retain it in a separate Docker volume. Decrypt SSH keys on the server when needed. Accept only private keys without passphrases in v1 and return only identifying metadata through key-management APIs.

Allow reuse across saved connections and block deletion while referenced. Include a clearly labeled demo-only key in the repository for the lab.

## Consequences

A database dump alone does not expose usable SSH private keys. Access to both volumes defeats that protection, and the running gateway must access plaintext keys to authenticate. Losing encryption material requires re-uploading keys; the app must report the problem rather than silently replacing encryption material for existing records.
