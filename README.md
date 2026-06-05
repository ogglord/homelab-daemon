# homelab-daemon

Service orchestrator and CLI for the homelab NixOS setup.

## Components

- **`homelab-daemon`** — privileged systemd service managing all homelab
  services (native systemd units and podman containers). Runs as root,
  listens on a Unix socket at `/run/homelab-daemon/daemon.sock`.
- **`homelab`** — CLI frontend for the daemon socket.
  `homelab status`, `homelab start <unit>`, etc.
- **`homelab-frontend`** — React/Vite web dashboard. Served by Caddy
  from the consuming NixOS flake.

## Quick start

```bash
nix develop          # go 1.26, node 24, gopls, golangci-lint
go build ./cmd/daemon/
cd frontend && npm run dev
```

See `docs/rebuild-workflow.md` for the full cycle.

## Architecture

```
cmd/daemon/       → homelab-daemon (HTTP server)
cmd/cli/          → homelab CLI
frontend/         → React/Vite web dashboard
api-types/        → Generated TypeScript types from pkg/api
pkg/api/          → Go API types (canonical, tygo generates TS from here)
pkg/logging/      → structured logging helpers
internal/         → private Go packages (cmdrunner, collector, bcachefs, etc.)
```
