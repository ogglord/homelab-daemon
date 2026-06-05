# homelab-daemon

Service orchestrator and CLI for the homelab NixOS setup.

## Components

- **homelab-daemon** — privileged systemd service that manages the lifecycle of all homelab services (native systemd units and podman containers). Runs as root, communicates over a Unix socket.
- **homelab** — CLI frontend for the daemon socket. Supports status, start/stop/restart, enable/disable, logs, secrets, and diagnostics.

## Build

```bash
nix build .#homelab-daemon
```

## Dev shell

```bash
nix develop
go build ./cmd/daemon/
go build ./cmd/homelab/
```
