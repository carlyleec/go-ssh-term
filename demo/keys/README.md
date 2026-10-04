# Local demo SSH key

This key pair is **intentionally public and demo-only**, including the private
key. Anyone with this repository can use it. Use it only for this project's
disposable local SSH lab; never authorize it on a real or publicly reachable host.

- `demo_ed25519` is the unencrypted OpenSSH Ed25519 private key to upload.
- `demo_ed25519.pub` is the matching public key for the lab hosts' authorized keys.

The bastion and both private targets authorize this identity for their `demo`
user. Browser terminals connect directly to the bastion or reach either private target
through a saved bastion connection.

## Upload

1. Follow the root README's [development](../../README.md#development-with-docker)
   or [demo](../../README.md#demo-with-docker) startup instructions, including
   migrations. Open the documented localhost URL for that mode.
2. Create an account with a passkey or sign in, then open **SSH Keys** from the navbar.
3. Choose **Upload SSH key**, enter a name such as **Local demo**, and select
   `demo/keys/demo_ed25519` as the **Private-key file**. Do not select the `.pub` file.
4. Choose **Upload key**. The saved entry should show this public fingerprint:

   ```text
   SHA256:gmYOO2ypscbxvYMU7+ZTPpZALzNOyCr++TGRXIQJjjA
   ```

The app accepts a single unencrypted Ed25519 private key in OpenSSH format, up to
16 KiB. This supplied key has no passphrase. The app lists only identifying
metadata and does not offer private-key downloads; the original demo file remains
available here for re-uploading.

## Storage and recovery

This SSH identity authenticates to lab hosts. It is separate from both your
login passkey and the secret application encryption material generated at
`ENCRYPTION_KEY_PATH`. Never replace that application key with this demo file or
commit the generated application key.

Uploaded private keys are encrypted in the SQLite volume using material in the
separate `encryption-key` volume. Keep both volumes to preserve usable uploads.
See [database storage and volume recovery](../../docs/configuration.md#storage-and-recovery)
for backup and missing-key recovery instructions. Losing the encryption material
requires restoring the matching backup or deliberately discarding unusable
records and re-uploading their original keys. Access to both volumes defeats the
protection against a database-only disclosure.
