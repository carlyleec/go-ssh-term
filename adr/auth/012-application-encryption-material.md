# Persist application encryption material separately and verify it at startup

Status: Accepted; supplements [SSH key storage](002-ssh-key-storage.md).

## Context

Stored SSH credentials must survive container recreation without putting their
decryption key in SQLite. Missing or incorrect material must prevent startup
instead of silently making existing credentials unreadable.

## Decision

Require `ENCRYPTION_KEY_PATH`, an absolute clean file path in a separate private
directory from the database. Both Compose modes mount the same `encryption-key`
volume at `/key-material`; storage initialization gives UID/GID 65532 ownership
and directory mode 0700. The migration tool does not mount this volume.

After verifying database migrations and before opening the HTTP listener, inspect
all stored SSH key records under the database operation deadline. If the material
is missing, generate 32 random bytes only when the SSH key table is empty. A
database read failure never permits generation. Reject malformed, inaccessible,
symlinked, or non-private key files, including when the database is empty.

Write a temporary mode-0600 file in the key directory, sync it, and publish it by
hard link without overwriting an existing file. Sync the directory and read the
published file. Existing material is reused rather than rotated automatically.
The persistent filesystem must support hard links and directory syncing.

Use AES-256-GCM through Go's `cipher.NewGCMWithRandomNonce`. The encrypted BLOB
format is a version byte (1), a 12-byte random nonce, ciphertext, and a 16-byte
authentication tag. Authenticate the byte string
`go-ssh-term:ssh-key:v1\x00<account UUID>\x00<key UUID>` as associated data, with
each `\x00` representing a NUL byte. This binds ciphertext to its account and
record. Reject unknown versions, invalid lengths, and authentication failures.
Random-nonce GCM permits fewer than 2^32 encryptions per application key; this
local application does not implement automatic rotation.

Startup authenticates/decrypts every record and clears the temporary plaintext
buffers. Any failure stops startup with a generic recovery message. Neither key
material, plaintext, ciphertext, nor library parsing details appear in errors or
logs. No decryption endpoint is exposed.

## Consequences

Keep both volumes in backups. A database copy alone does not reveal usable SSH
private keys; possession of both volumes or access to the running process defeats
that protection. Losing the key volume requires restoring the matching material
or explicitly discarding encrypted credentials and uploading their originals
again. The app never deletes those records as part of startup recovery.

Verification is a startup check, not continuous monitoring of the key file. The
single-process deployment assumption remains. Authentication is implemented with
the standard library; upload encryption and its nonce tests are completed with
the encryption implementation task.
