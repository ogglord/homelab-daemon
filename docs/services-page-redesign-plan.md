# ServicesPage Redesign + Orchestration Hardening

## Context

The ServicesPage currently shows a minimal table (name, type, status, description, autostart, actions). Inspired by Unraid's Docker Containers page, the user wants:
1. **Service icons** (already in `icon_url` on `ServiceInfo`, used on Overview widget but not in the table)
2. **Quick log link** per row
3. **Port mappings** surfaced in the UI (currently not in the API at all)
4. **Caddy/homepage URL** as a clickable link per row
5. A **view toggle** (normal/expert) similar to Unraid's "BASIC VIEW" button
6. **Orchestration stability** — several gaps were found in the daemon's restart logic

---

## Part 1 — Daemon: Port Mappings in API

### `pkg/api/services.go`
Add a `PortMapping` struct and `PortMappings` field to `ServiceInfo`:

```go
type PortMapping struct {
    ContainerPort int    `json:"container_port"`
    HostPort      int    `json:"host_port"`
    Protocol      string `json:"protocol"`          // "tcp" | "udp"
    HostIP        string `json:"host_ip,omitempty"` // omit if "0.0.0.0" or ""
}
// Add to ServiceInfo:
PortMappings []PortMapping `json:"port_mappings,omitempty"`
```

### `internal/updates/updates.go`
Switch `podman ps -a` from tab-delimited `--format "{{.Names}}\t..."` to `--format json`. Parse the structured JSON (which already includes `Ports[]` with `host_ip`, `container_port`, `host_port`, `protocol`). Add `Ports []api.PortMapping` to `MetadataEntry`. Populate during `runChecks()`.

```go
type podmanPsEntry struct {
    Names  []string          `json:"Names"`
    Image  string            `json:"Image"`
    Labels map[string]string `json:"Labels"`
    Ports  []struct {
        HostIP        string `json:"host_ip"`
        ContainerPort int    `json:"container_port"`
        HostPort      int    `json:"host_port"`
        Protocol      string `json:"protocol"`
    } `json:"Ports"`
}
```

### `cmd/daemon/api.go`
Populate `PortMappings` from `MetadataEntry.Ports` in the two `ServiceInfo`-build loops (the `/api/services/merged` handler ~line 1078 and the `/api/v1/overview` handler ~line 1233). Also fix the older endpoint which currently omits `IconURL`, `HomepageURL`, `BlockedReason`, `RequiresMounts`.

### Run `make types`
After Go changes so `PortMapping` and `port_mappings` appear in `frontend/src/types.gen.ts`.

---

## Part 2 — Orchestration Hardening

Five gaps were found in monitor.go/api.go. Listed by severity:

### Fix A — `depends_on` not checked during monitor restarts (🔴)
**File:** `cmd/daemon/monitor.go` — `restart()` function (~line 469)

Currently `depends_on` is only verified during `boot()`, not in the 5-second monitor loop restart path. If a service's dependency is dead, the monitor will still attempt to restart the dependent.

**Fix:** Before calling `startUnit()` inside `restart()`, check all `svc.DependsOn` entries with `isActive()`. If any dep is inactive, skip the restart attempt (don't record a failure — it's a constraint miss, not a crash).

```go
for _, dep := range svc.DependsOn {
    if !isActive(dep) {
        monitorLog.Info("restart skipped — dependency inactive", "unit", svc.Unit, "dep", dep)
        return
    }
}
```

### Fix B — Race between API stop and monitor restart (🔴)
**File:** `cmd/daemon/monitor.go` + `cmd/daemon/api.go`

The monitor loop and API handlers both call `startUnit()`/`stopUnit()` without mutual exclusion. An API stop can race with a concurrent monitor restart.

**Fix:** Add a package-level `sync.Mutex` (`var unitOpMu sync.Mutex`) guarding all calls to `startUnit()` and `stopUnit()`. Both the monitor loop's `restart()` and the API's start/stop handlers acquire this mutex before issuing the systemctl call. The mutex should be per-unit (a `sync.Map` of mutexes keyed by unit name) to avoid global serialization.

### Fix C — Dependency re-check before `startUnit()` in boot (🔴)
**File:** `cmd/daemon/monitor.go` — `boot()` function (~line 327)

After `waitActive(dep, 2*min)` succeeds, a dependency might crash before the dependent is started. Add a final `isActive(dep)` check immediately before `startUnit()`.

### Fix D — User-stopped flag not cleared on re-enable (🟡)
**File:** `cmd/daemon/api.go` — PATCH `/api/config/{unit}` handler (~line 441)

When a service is re-enabled via config PATCH (`enabled: true`), `IsUserStopped()` is not cleared. The monitor loop then sees it as user-stopped and refuses to restart it.

**Fix:** When `payload.Enabled != nil && *payload.Enabled == true`, call `state.SetUserStopped(unit, false)`.

### Fix E — Circuit breaker persists after disable/re-enable (🟡)
**File:** `cmd/daemon/monitor.go` — config reload or re-enable path

When a service that has hit max backoff is disabled and then re-enabled, the circuit breaker's backoff window still blocks restarts.

**Fix:** When a service is re-enabled (same place as Fix D), call `breaker.Reset(unit)` so the backoff state is cleared and the service gets a fresh start.

---

## Part 3 — Frontend: View Toggle + Table Redesign

### View toggle
Add a `viewMode: "normal" | "expert"` state to `ServicesPage`, persisted to `localStorage` key `homelab-services-view`. Render a toggle button in the `PageShell` header area (mirroring Unraid's "BASIC VIEW" button).

- **Normal mode**: Icon-first card grid (2–4 per row), each card shows icon + name + StateBadge + quick start/stop button + log icon + external link icon. Similar to `ServicesWidget` but larger cards with controls.
- **Expert mode**: Full Unraid-inspired table (see columns below).

### Expert mode table columns

| Col | Header | Width | Content |
|---|---|---|---|
| 1 | _(none)_ | 44px | 32px icon, greyed+desaturated if inactive |
| 2 | Name | 160px | Name + orange update dot + external-link icon if `homepage_url` |
| 3 | Status | 176px | `StateBadge` (unchanged) |
| 4 | Web / Ports | flexible | Port chips + homepage URL link |
| 5 | AutoStart | 96px | Switch (unchanged) |
| 6 | _(none)_ | 56px | Quick-logs button + `RowActions` dropdown |

"Type" and "Description" columns removed — Description moves to icon tooltip.

### "Web / Ports" cell
- Clickable chip with truncated hostname from `homepage_url` (if set) + ExternalLink icon
- Port pills in monospace: `8096 → 8096/tcp`  
- Show max 3 pills inline; `+N more` badge with tooltip for overflow
- Empty cell if no URL and no ports

### Normal mode card design
Each card (roughly `w-48 h-auto`):
```
[32px icon]  Name              [status dot]
             active / inactive
             [Log] [Open ↗]  [Start/Stop]
```
Context menu still available via right-click (reuse existing `useRowContextMenu`).

### Quick-logs button
In expert mode actions cell, render a `ScrollText` lucide icon button **before** `RowActions` that calls `showLogs(unit)` directly.

### `ServiceRow` / `toRow()` additions
```ts
icon_url: svc.icon_url ?? "",
homepage_url: svc.homepage_url ?? "",
port_mappings: svc.port_mappings ?? [],
```

### New file: `ServicesPage/PortChips.tsx`
Presentational component — accepts `ports: PortMapping[]` and `homepageUrl?: string`, renders the compact chip list with overflow handling.

### New file: `ServicesPage/ServiceCard.tsx`
Card component for normal mode view. Accepts a `ServiceRow` + callbacks matching the existing `ServiceActionsCallbacks` interface. Keeps all action logic in `ServicesPage.tsx`.

---

## Stability notes
- **Port data is cached** in the updates module (6h refresh) — zero polling overhead added.
- **No new API endpoints** — port data piggybacks on existing `ServiceInfo`.
- **Graceful fallback** — `port_mappings` is `omitempty`; old daemon → empty array → no UI change.
- **No state management changes** — SharedPoller, useOverview, optimistic updates for AutoStart/stop-disable all stay exactly as-is.
- **Orchestration fixes are non-breaking** — guard clauses only, no changed semantics for healthy services.

---

## Verification
1. `go build ./cmd/daemon/ ./cmd/cli/` — compile clean
2. `make preflight` — types → vet → build
3. `cd frontend && npm run build` — TypeScript clean
4. Confirm `GET /api/v1/overview` includes `port_mappings` for docker services
5. UI: toggle between normal/expert modes; icon renders with fallback; port chips appear; homepage link opens; quick-logs button works
6. Orchestration: verify depends_on check fires in monitor loop log output; verify re-enabling a service clears user-stopped flag
