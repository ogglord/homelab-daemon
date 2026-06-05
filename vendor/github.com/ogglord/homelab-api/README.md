# pkg/api — shared wire contract

This is the single source of truth for the JSON shapes spoken between
`homelab-daemon` (privileged, owns the unix socket) and `homelab-dash`
(unprivileged UI). It also feeds the frontend TypeScript types.

## Layout

```
pkg/api/
  version.go   — Version, APIPrefix, SocketPath, VersionResponse
  common.go    — SuccessResponse, BulkResult, ErrorResponse
  services.go  — Service, ServiceStatus, PatchServiceRequest/Response
  backups.go   — Backup, BackupStatus, PatchBackupRequest/Response
  storage.go   — Pool, Disk, PoolUsage, StorageStatus, mount/check/balance reqs
  secrets.go   — SecretEntry, SecretStatus, SecretsListResponse, SecretSetRequest
  vms.go       — VMInfo + VMState* consts
  updates.go   — UpdateInfo, MetadataEntry, UpdatesStatus
  tygo.yaml    — config for TS code-gen
```

## Module path

`github.com/ogglord/homelab-api` — matches the existing `homelab-*`
naming. Both consumers reference it via a `replace` directive pointing
at `../pkg/api`, and via the top-level `go.work` for local builds.

## Stdlib only

This module deliberately has **no third-party dependencies**. Keep it
that way: anything else is rendered into JSON elsewhere and consumed
elsewhere — types are passive.

## Versioning

`Version = "v1"`. Bump on breaking JSON changes. The daemon mounts the
same handlers on both `/api/...` (legacy) and `/api/v1/...` during the
transition; new clients should prefer the prefixed form.

## Regenerating TypeScript

```
go install github.com/gzuidhof/tygo@latest
tygo generate    # uses pkg/api/tygo.yaml
```

The current `homelab-dash/frontend/src/types.ts` is hand-maintained
and is being kept as-is during this transition (see REVIEW.md REC #1).
Once `tygo` is wired into CI, that file will be replaced by the
generated `types.gen.ts`.
