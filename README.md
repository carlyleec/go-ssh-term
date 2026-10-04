# Browser SSH Gateway

A Go and React browser SSH gateway built as a learning and portfolio project.
Sign in with a passkey, upload an SSH key, save or import connections, and open
an interactive terminal directly or through a bastion.

The frontend uses React, TanStack Router/Query/Form, daisyUI, and xterm.js.
Go handles authentication and SSH transport; SQLite stores accounts, encrypted
SSH keys, saved connections, and connection events.

## Current capabilities

- Passkey registration and sign-in, with account-scoped keys and connections.
- Separate Workspace, Connections, and SSH Keys pages with right-side drawers.
- Explicit SSH host fingerprint approval and single-hop bastion connections.
- Reviewed import of a limited SSH config subset.
- One active browser terminal, with explicit reconnect to a fresh shell.

Terminal tabs and automatic recovery are still planned. Switching between the
three signed-in sections preserves the active shell; refresh, logout, or session
expiry ends it. Reconnecting does not restore commands, output, or working directory.

This is a local demo, not a production-hardened service. There is one passkey per
account and no account recovery. SSH uploads currently support unencrypted
OpenSSH Ed25519 keys up to 16 KiB. Password SSH login, multiple jumps, file transfer,
and terminal recording are outside the current scope.

## Demo with Docker

Requirements: Docker with Compose, Make, and a browser that supports passkeys.
No host Go or Bun installation is needed. Run this command from the repository
root:

```sh
make demo
```

Open [localhost:8080](http://localhost:8080). Compose starts the app, a bastion,
and two private SSH targets. Only the bastion can reach those targets.

Leave `BROWSER_ORIGIN` unset for the mode-specific default. If you have an existing
`.env` override, it must match the browser URL exactly. Use `localhost` consistently
for passkeys. See [configuration](docs/configuration.md) for other settings.

Useful commands:

```sh
docker compose ps
docker compose logs -f app
docker compose down          # Stop containers; keep stored data
```

Run `make demo` again after pulling changes. It stops the app, initializes storage,
applies pending migrations, builds, and starts the demo, waiting for readiness.
Each step must succeed before the next runs; stored data is preserved.

## Try the SSH lab

1. Register with a display name and a passkey, or sign in.
2. Open **SSH Keys**, choose **Upload SSH key**, and upload
   [demo/keys/demo_ed25519](demo/keys/demo_ed25519). This intentionally public key
   is only for the disposable lab; never authorize it on a real host.
3. Open **Connections** → **Import SSH config** and select
   [demo/ssh_config](demo/ssh_config). Keep all three hosts selected and map each
   identity to your uploaded key.
4. Choose **Check selection**, review the settings, then **Confirm import**.
   Nothing is saved until confirmation, and existing names are never overwritten.
5. Open **Workspace** → **Connect**, select `bastion`, and compare its fingerprint
   with the lab's fingerprint before approving it. Choose **Open terminal**.
6. Run `cat /host-info.txt`. Close the terminal and repeat with `target-1` and
   `target-2`; each prints its own identity. Target connections verify both the
   bastion and target fingerprints.
7. Close the terminal or choose **Sign out** from the account dropdown.

Get the lab fingerprints for comparison:

```sh
for host in bastion target-1 target-2; do
  docker compose exec "$host" sh -c 'for key in /var/lib/ssh-host-keys/*_key.pub; do ssh-keygen -lf "$key"; done'
done
```

A changed host key blocks connection. Verify the change before choosing
**Reset host trust**, then inspect and approve the replacement fingerprint.
Resetting trust never approves the new key automatically.

Import supports `Host`, `HostName`, `User`, `Port`, `IdentityFile`, and one
`ProxyJump`. It does not execute directives or read referenced identity files.
See the [supported syntax and limits](adr/api/022-ssh-config-import.md).

## Development with Docker

With Docker and Make installed:

```sh
make migrate
make up
```

Open [localhost:5173](http://localhost:5173). Vite serves the frontend and proxies
API and terminal traffic to Go. Air restarts Go after backend changes; those
restarts end active shells.

Demo and development share stored data when using the same Compose project.
Stop one mode before starting the other: `docker compose down` for demo or
`make down` for development. Do not add `--volumes` when switching modes.

For local editor tooling, install asdf and its Go/Bun plugins, then run
`make setup`. Versions are pinned in [.tool-versions](.tool-versions).

## Tests and checks

With development services running, use another terminal:

```sh
make check          # Go/frontend tests, Biome, and TypeScript
make build          # Production frontend build in a temporary container
make format         # Format frontend files
make logs           # Follow development logs
make help           # List available commands
```

See [development guidance](docs/development.md) for race tests, the real SSH lab
integration check, dependency updates, generated contracts, and host tooling.

## Data and reset

Ordinary shutdown and container recreation preserve accounts, connections,
uploaded keys, and lab host identities. Keep both the `sqlite-data` and
`encryption-key` volumes: the database alone cannot decrypt uploaded SSH keys.
See [storage and recovery](docs/configuration.md#storage-and-recovery).

To deliberately start over, the following command **deletes application data,
the application encryption key, lab host identities, and container build caches**:

```sh
docker compose -f compose.yaml -f compose.dev.yaml down --volumes
```

Use the same Compose project name as startup. Then repeat the migration-first
startup for your chosen mode. This does not remove source files or host editor
dependencies.

## Project documentation

- [Product requirements](PRD.md): scope, behavior, and exclusions.
- [Development plan](DEV_PLAN.md): implementation progress and remaining work.
- [Architecture decisions](adr/README.md): design choices and protocol details.
- [OpenAPI schema](openapi/api.json): HTTP endpoints and request/response contracts.
- [Development guide](docs/development.md): tooling and verification workflows.
- [Configuration guide](docs/configuration.md): settings, storage, and troubleshooting.
