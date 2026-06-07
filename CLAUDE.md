# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build / test / lint

```bash
nix develop                        # dev shell: go 1.26, node 24, gopls, golangci-lint, tygo
go build ./cmd/daemon/ ./cmd/cli/  # build both Go binaries
go vet ./...                       # vet
go test ./...                      # run tests (cmdrunner + collector packages)
make preflight                     # types → vet → build (full CI gate)
make deploy                        # preflight + commit + push
```

Frontend:
```bash
cd frontend && npm run dev          # Vite dev server with hot reload
cd frontend && npm run build        # tsc + vite build
```

Regenerate TypeScript types after changing `pkg/api/*.go`:
```bash
make types   # tygo generates api-types/index.ts → copied to frontend/src/types.gen.ts
```

Update Nix hashes when deps change:
```bash
./scripts/update-hashes.sh daemon    # vendorHash
./scripts/update-hashes.sh frontend  # npmDepsHash
./scripts/update-hashes.sh all       # both
```

## Architecture

Three artifacts built from one repo:

| Artifact | Entrypoint | Runtime |
|---|---|---|
| `homelab-daemon` | `cmd/daemon/` | Root systemd service, Unix socket at `/run/homelab-daemon/daemon.sock` |
| `homelab` CLI | `cmd/cli/` | User CLI, talks to daemon via Unix socket |
| `homelab-frontend` | `frontend/` | React/Vite SPA served by Caddy in the consuming NixOS flake |

### Daemon (`cmd/daemon/`)

Single-package Go binary. All files in one flat package — no subdirectories.

**Startup sequence** in `main.go`:
1. Health checks (state dir writable, config readable, systemctl/podman on PATH)
2. Load `services.yaml` config + NixOS managed-units registry, filter stale entries
3. Load persisted state (`state.json`): user-stopped flags, kernel boot-id
4. Crash detection: if `.clean-shutdown` marker missing from last run → notify
5. Boot sequence (only on new kernel boot-id, not daemon restarts):
   - Auto-mount bcachefs pools from config
   - Verify running services against constraints
   - Start enabled services in `order` sequence respecting `depends_on`/`boot_delay`
6. Launch background workers via `errgroup`: updates checker, stats collector, metrics server, API server, monitor loop

**Key subsystems:**

- **`api.go`** — HTTP server on Unix socket. Routes: `/api/status`, `/api/start|stop/:unit`, `/api/start-all|stop-all`, `/api/reload`, `/api/config`, `/api/backups`, `/api/log-viewer-config`, pi-web proxies, Prometheus metrics. Auth via socket group permissions (0660, group homelab-daemon).

- **`monitor.go`** — Restart loop with circuit breaker. Backoff schedule: 0s → 30s → 2m → 10m → 1h. Tracks per-unit consecutive failures. Respects restart policies (`no`, `on-failure`, `unless-stopped`, `always`).

- **`scheduler.go`** — Cron-based backup runner using `robfig/cron`. Pings healthchecks.io on start/success/fail. Sends SMTP alerts on backup failure.

- **`config.go`** — YAML config schema (`services.yaml`). `Service` struct has unit name, enabled, order, boot_delay, depends_on, requires_mount, restart policy, restart_delay. `Backup` struct has schedule, depends_on, requires_mount, healthcheck UUID, pause_service. Schema migration via version field.

- **`state.go`** — Persistent state (`/var/lib/homelab-daemon/state.json`). Tracks user-stopped units (survives reboots) and kernel boot-id (to distinguish real boots from daemon restarts). `IsFirstStartAfterBoot()` compares `/proc/sys/kernel/random/boot_id` against persisted value.

- **`config_writer.go`** — Writes config changes back to `services.yaml` preserving comments and formatting via a line-by-line merge strategy.

- **`merge.go`** — `merge-config` subcommand merges user overrides (`~/.config/homelab/services.yaml`) into the active config with copy-on-write tempfile.

- **`middleware.go`** — Request ID propagation (reads `X-Request-Id` header or generates one), structured request logging (5xx→ERROR, 4xx→WARN), SO_PEERCRED listener wrapper (credential extraction retained but guard is no-op since Phase 3).

- **`metrics.go`** — Prometheus `/metrics` on loopback `127.0.0.1:9101` (override via `HOMELAB_METRICS_ADDR`). Tracks HTTP request counts + latency. Dedicated registry with Go + process collectors.

### Internal packages

- **`internal/cmdrunner/`** — Shell command execution with caller module tracking, optional user impersonation, streaming line handlers, JSON output helpers.
- **`internal/collector/`** — Host stats polling (2s interval): CPU, memory, disk, network, top processes, GPU (Intel iGPU via `intel_gpu_top`), host info. Exposed via API as JSON.
- **`internal/notifier/`** — SMTP email alerts with rate-limiting cooldowns. Templates for daemon crash, backup failure, service failure.
- **`internal/updates/`** — Podman container update checker. Scans running containers with `podman auto-update --dry-run`.
- **`internal/vms/`** — libvirt VM enumeration. Build-tagged `linux` only; `vms_stub.go` for other platforms.
- **`internal/storage/bcachefs/`** — bcachefs pool discovery, mount/unmount, subvolume management.

### API types (`pkg/api/`)

Go structs in `pkg/api/*.go` are the canonical source of truth for the daemon↔frontend wire contract. TypeScript types are generated via `tygo` and written to `api-types/index.ts`, then copied to `frontend/src/types.gen.ts`. Never edit generated files by hand.

### Logging (`pkg/logging/`)

Centralized `slog` logger factory. All Go packages must use `logging.Logger(module)` — direct `slog.Default()` calls forbidden. Canonical module names enforced via panic on unknown modules. Context keys (`CtxKeyReqID`, `CtxKeyPeerUID`) exported here so all packages share the same key identity.

### Frontend (`frontend/`)

React 19 + Vite 8 + TypeScript 6. Tailwind CSS v4 with `tailwindcss-react-aria-components` plugin. React Router v7 for routing. `react-aria-components` for accessible UI primitives. Custom shadcn/ui-style component library in `src/components/ui/`.

Pages: Overview (widget grid), Services (table + config sheets), Backups, Storage (bcachefs pools + subvolumes), Logs (log viewer), Diagnostics, VMs, Secrets, Iframes.

Widgets on the overview dashboard are registered in `src/widgets/registry.ts`. They consume data from the daemon's `/api/status` polling (default 5s, configurable).

### Nix

`flake.nix` builds three packages: `daemon`, `cli`, `frontend`. Exposes a `nixosModules.default` that adds overlays and imports `module.nix` (the self-contained NixOS systemd module). Dev shell includes go, gopls, golangci-lint, nodejs, tygo, nixfmt-rfc-style.

Local Go modules resolved via `replace` directives in `go.mod` (no `go.work`):
- `github.com/ogglord/homelab-api` → `./pkg/api`
- `github.com/ogglord/homelab-logging` → `./pkg/logging`

### Pre-commit hooks

`.githooks/` directory auto-updates `vendorHash`/`npmDepsHash` in `flake.nix`. Nix dev shell auto-configures `git config core.hooksPath .githooks`.
