# Configuration and troubleshooting

See the [README](../README.md) for startup commands. Copy
[.env.example](../.env.example) to `.env` only if you need to customize Compose
settings. Go reads process environment variables; it does not load `.env` itself.

## Settings

| Setting | Default | Meaning |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | Go listen address. The demo fixes its internal address at `:8080`. |
| `DATABASE_PATH` | Required by Go; Compose uses `/data/gateway.db` | Absolute, clean path to an existing, migrated SQLite file. |
| `ENCRYPTION_KEY_PATH` | Required by Go; Compose uses `/key-material/application.key` | Absolute path in a separate private, persistent directory. |
| `BROWSER_ORIGIN` | Demo: `http://localhost:8080`; development: `http://localhost:5173` | Exact browser origin, without a trailing slash, path, query, or credentials. HTTP requires `localhost`; other domains require HTTPS. IP addresses are not supported. |
| `SESSION_LIFETIME` | `12h` | Absolute login-session lifetime, at least one second. |
| `CHALLENGE_LIFETIME` | `5m` | Passkey challenge lifetime, at least one millisecond. |
| `SHUTDOWN_TIMEOUT` | `5s` | Positive Go duration for graceful shutdown. |
| `APP_STOP_GRACE_PERIOD` | `15s` | Compose's outer shutdown deadline; must exceed the app deadline. |

Leave `BROWSER_ORIGIN` unset to use the mode-specific Compose defaults. Passkeys
are scoped to the origin hostname, while API origin checks also include scheme
and port. Use `localhost` consistently when switching between demo and development.
Recreate the app after changing environment settings:

```sh
docker compose -f compose.yaml -f compose.dev.yaml up -d --force-recreate app
```

For demo mode, omit `-f compose.yaml -f compose.dev.yaml`. If changing the
container HTTP port in development, also update Compose's port mapping and Vite's
proxy target. Use an unspecified listen host such as `:8080` to accept forwarded
container traffic.

Air allows ten seconds for Go to stop. If increasing `SHUTDOWN_TIMEOUT`, also
increase Air's `kill_delay` and Compose's grace period.

## Storage and recovery

Demo and development share persistent volumes when using the same Compose project:

| Volume | Contents |
| --- | --- |
| `sqlite-data` | Accounts, sessions, encrypted SSH keys, connections, host trust, and connection events. |
| `encryption-key` | Secret application key used to decrypt uploaded SSH keys. |
| `bastion-host-keys`, `target-1-host-keys`, `target-2-host-keys` | Persistent identities of the lab SSH hosts. |

Stop one mode before starting another. Use one app process and a local filesystem.
Keep the SQLite database, `-wal`, and `-shm` files together in the directory volume.
Ordinary shutdown and recreation preserve these volumes.

Back up both the database and its matching encryption key. For a file-level copy,
stop the app first and preserve the entire database directory. The database alone
cannot decrypt uploaded SSH keys; access to both volumes defeats that separation.
The public [demo SSH key](../demo/keys/README.md) is not the application encryption key.

Storage initialization grants UID/GID 65532 access to the directories used by the
app and dbmate. Custom database paths must be in writable mounts shared with
migration tooling. The encryption-key directory must be separate, private (0700),
and persistent; its key file uses mode 0600.

At first startup, Go generates application key material only if no SSH key records
exist. Startup checks that stored keys can be decrypted. Missing, malformed, or
incorrect key material is not silently replaced. Restore the matching backup and
check permissions. Without that key, encrypted records cannot be recovered; the
original SSH keys would need to be uploaded again after deliberately clearing the
unusable records. Automatic recovery is not implemented.

To verify persistence without erasing data, upload a key, note its fingerprint,
and recreate the app:

```sh
# Development
docker compose -f compose.yaml -f compose.dev.yaml up -d --no-deps --force-recreate app

# Demo: use this instead
docker compose up -d --no-deps --force-recreate --wait app
```

Reopen SSH Keys and check that the saved metadata remains. See
[Data and reset](../README.md#data-and-reset) only when intentionally starting over.
The reset command removes declared Compose volumes, not custom host storage or
undeclared volumes from older configurations.

## Troubleshooting

### The server will not start after pulling changes

Check logs for missing or pending migrations. In development, run `make migrate`
and then `make up`. For demo mode, run `make demo` to stop the app, initialize
storage, apply migrations, and rebuild/start the demo.
The server validates migration history at startup and never applies migrations.

### Passkey or origin checks fail

Match `BROWSER_ORIGIN` to the exact URL you opened. A `.env` override for port 5173
will also affect demo mode unless changed or removed. Avoid switching between
`localhost` and an IP address. If another process occupies the IPv6 localhost port,
stop it or choose an unused port rather than changing to an IP-based browser URL.

Passkey challenges are process-local. After an app/Air restart or an expired
challenge, begin sign-in or registration again. If registration saved the account
but session creation failed, try signing in with the newly created passkey.
Account recovery and adding replacement passkeys are not supported.

### The container is running but the UI cannot reach Go

Check `make logs` for Air build errors. The development health check is disabled
because Vite does not require a compiled frontend. `/api/readyz` still checks both
SQLite and compiled frontend assets, so it can return 503 until `make build` runs.
In demo mode, readiness returns 204 when both checks succeed. Docker marks failing
health checks unhealthy but does not automatically restart the container.

### A lab host fingerprint changed

Ordinary container recreation preserves host identities. Removing a host-key
volume creates a new identity on the next start. Compare fingerprints using the
[lab walkthrough](../README.md#try-the-ssh-lab), investigate the change, and only
then reset trust and approve the replacement. Both hops require verification when
connecting through a bastion.

### Import reports conflicts or unresolved references

Existing connection names are never overwritten. Deselect conflicting entries,
or rename them and their `ProxyJump` references in the file. If the bastion already
exists, deselect its imported entry and select the existing connection as the
jump. Choose uploaded keys for the selected entries, then check the selection
again. Import does not read local paths named by `IdentityFile`.

See the [import syntax](../adr/api/022-ssh-config-import.md) for supported directives
and limits, and the [architecture index](../adr/README.md) for implementation details.
