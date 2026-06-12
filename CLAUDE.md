# CLAUDE.md

## WHAT

Three artifacts from one repo:

| Artifact | Entrypoint | Role |
|---|---|---|
| `homelab-daemon` | `cmd/daemon/` | Root systemd service, Unix socket `/run/homelab-daemon/daemon.sock` |
| `homelab` CLI | `cmd/cli/` | User CLI, talks to daemon via Unix socket |
| `homelab-frontend` | `frontend/` | React/Vite SPA served by Caddy |

**Daemon subsystems** (all flat in `cmd/daemon/`): `api.go` HTTP server · `monitor.go` restart loop w/ circuit breaker · `scheduler.go` cron backup runner · `state.go` persistent state (`state.json`: user-stopped flags, boot-id, backup last-run times) · `config.go` YAML schema · `config_writer.go` comment-preserving config writes · `metrics.go` Prometheus on `127.0.0.1:9101`.

**Internal packages**: `cmdrunner` shell exec · `collector` host stats (CPU/mem/disk/net/GPU) · `notifier` SMTP alerts · `updates` podman update checker · `vms` libvirt (linux-only) · `storage/bcachefs` pool management.

**API contract**: Go structs in `pkg/api/*.go` → TypeScript via `tygo` → `frontend/src/types.gen.ts`. Never edit generated files by hand.

**Frontend**: React 19 + Vite 8 + TypeScript 6 + Tailwind v4 + React Router v7 + react-aria-components. Widgets registered in `src/widgets/registry.ts`, consume `/api/status` (5s poll).

## WHY

- `state.go` persists across daemon restarts (not just reboots) — `IsFirstStartAfterBoot()` uses `/proc/sys/kernel/random/boot_id` to distinguish real boots from daemon restarts.
- Go modules use `replace` directives (no `go.work`): `homelab-api` → `./pkg/api`, `homelab-logging` → `./pkg/logging`.
- Pre-commit hooks (`.githooks/`) auto-update `vendorHash`/`npmDepsHash` in `flake.nix`.
- All logging via `logging.Logger(module)` — direct `slog.Default()` forbidden.

## HOW

```bash
# Dev shell (go 1.26, node 24, gopls, golangci-lint, tygo)
nix develop

# Build
go build ./cmd/daemon/ ./cmd/cli/
cd frontend && npm run build

# Test / lint
go test ./...
go vet ./...

# Full CI gate
make preflight          # types → vet → build

# Deploy (preflight + commit + push; then on NixOS host:)
make deploy             # nix flake update homelab-daemon && nh os switch .

# Regenerate TS types after changing pkg/api/*.go
make types

# Update Nix hashes after dep changes
./scripts/update-hashes.sh daemon    # vendorHash
./scripts/update-hashes.sh frontend  # npmDepsHash
./scripts/update-hashes.sh all       # both
```
