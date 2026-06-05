# homelab-daemon — agents playbook

This repo builds three things:
- **`homelab-daemon`** — privileged orchestrator (cmd/daemon)
- **`homelab`** — CLI tool (cmd/cli)
- **`homelab-frontend`** — React/Vite web dashboard (frontend/)

## Quick reference

```bash
# Enter dev shell
nix develop                    # go 1.26, node 24, tygo, gopls

# Build everything
go build ./cmd/daemon/
go build ./cmd/cli/

# Full pre-flight (types → vet → build)
make preflight

# Deploy (types → vet → build → commit → push)
make deploy
```

## TypeScript types from Go

API types are defined in `pkg/api/*.go`. Regenerate TypeScript after changing them:

```bash
make types
# Updates: api-types/index.ts + frontend/src/types.gen.ts
```

Never edit `api-types/index.ts` or `frontend/src/types.gen.ts` by hand.

## Hash mismatches (vendor / npm)

When Go deps or npm deps change, `vendorHash` or `npmDepsHash` will fail:

```bash
./scripts/update-hashes.sh     # auto-compute and patch flake.nix
```

## Structure

```
cmd/daemon/          → homelab-daemon server
cmd/cli/             → homelab CLI
frontend/            → React/Vite web dashboard
api-types/           → generated TypeScript from pkg/api
pkg/api/             → Go API types (source of truth, tygo generates TS)
pkg/logging/         → structured logging
internal/            → private Go packages
module.nix           → NixOS module (self-contained)
flake.nix            → builds daemon + CLI + frontend
Makefile             → types / vendor / preflight / deploy targets
scripts/             → update-hashes.sh, etc.
```

## Consuming flake updates

After pushing, the nixos repo needs:
```bash
nix flake update homelab-daemon
nh os switch .
```
Or use the bundled script: `sudo ./scripts/update-daemon.sh` in the nixos repo.
