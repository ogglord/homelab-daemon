# CLI Improvements & Homelab Doctor CI Integration

**Date:** 2026-06-13  
**Status:** Approved

---

## Overview

Two related improvements:

1. **CLI restructure** — migrate `cmd/cli/main.go` from manual string dispatch to `urfave/cli` v2, clean up inconsistencies, add missing commands.
2. **Doctor CI integration** — extract doctor logic into `internal/doctor`, make it scriptable (`--json`, `--fail-on-error`), and wire it into NixOS activation via an activation script + oneshot systemd service with SMTP notification on failure.

---

## Section 1: CLI Structure

### Framework

Migrate to `urfave/cli` v2. No other CLI framework considered — urfave/cli v2 is lightweight, stdlib-friendly, and requires no code generation.

### Command Hierarchy

```
homelab services status
homelab services start <unit>
homelab services stop <unit>
homelab services restart <unit>
homelab services enable <unit>
homelab services disable <unit>
homelab services logs <unit>

homelab backup status
homelab backup run <name>

homelab secret list
homelab secret add <name>       # NEW: declare + set a new secret (prompts description + value); calls PUT /api/secrets/<name> with {description, value}
homelab secret set [name]       # existing: rotate value only
homelab secret set              # interactive picker (unchanged)

homelab config show             # NEW: dump resolved config as JSON
homelab merge-config [flags]    # existing, unchanged

homelab daemon status           # NEW: lightweight ping to socket (not full doctor)

homelab doctor [flags]          # see Section 2
homelab doctor notify           # read JSON report from stdin, send SMTP if failures present

homelab version
```

**Top-level aliases preserved:**
- `homelab status` → `homelab services status`
- `homelab doctor` stays at top level

### Bug fixes

- `handleRestart` currently calls `systemctl restart` directly, bypassing the daemon socket. Fix: route through `POST /api/restart/<unit>` (add endpoint to daemon) to match `start`/`stop`.
- Remove the 100ms sleep in `handleDoctor`.

### Error handling

Extract repeated `os.Exit(1)` pattern into a single helper:

```go
func die(format string, args ...any) {
    fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
    os.Exit(1)
}
```

### Global flags

- `--json` — all commands that produce structured output (status, doctor, backup status, secret list) emit JSON instead of tabwriter output. Passed via `cli.Context`.

---

## Section 2: Doctor Internals

### New package: `internal/doctor`

Extract all check logic from `handleDoctor()` in `cmd/cli/main.go` into `internal/doctor/doctor.go`.

### Types

```go
type Check struct {
    Name string        // slug: "socket", "daemon", "postgres", etc.
    Run  func() Result
}

type Result struct {
    Name   string `json:"name"`
    OK     bool   `json:"ok"`
    Detail string `json:"detail,omitempty"`
    Fix    string `json:"fix,omitempty"`
}

type Report struct {
    Passed  int      `json:"passed"`
    Failed  int      `json:"failed"`
    Results []Result `json:"results"`
}
```

### Check registry

| Slug | Check |
|---|---|
| `socket` | Daemon Unix socket exists at `/run/homelab-daemon/daemon.sock` |
| `daemon` | `homelab-daemon.service` is active |
| `postgres` | `postgresql.service` is active |
| `caddy` | `caddy.service` is active |
| `dashboard` | HTTPS GET to dashboard URL returns HTTP 200 |
| `dashboard-pages` | All SPA routes return HTTP 200 + valid HTML |
| `disk` | `/`, `/cache`, `/pool` all under 90% used |
| `systemd-units` | No failed systemd units |

**Dashboard URL**: read from daemon via `GET /api/status` response (not hardcoded). Falls back to `https://dash.cignl.cc` if daemon unreachable.

### CLI flags for `homelab doctor`

| Flag | Behaviour |
|---|---|
| `--json` | Print `Report` as JSON, always exit 0 |
| `--fail-on-error` | Exit 1 if `Report.Failed > 0` |
| `--check <slug>` | Run only named checks (comma-separated, e.g. `--check disk,daemon`) |

`--json` and `--fail-on-error` are composable: `--json --fail-on-error` emits JSON and exits 1 on failure.

### Notification

New function in `internal/doctor`:

```go
func Notify(report Report, cfg NotifyConfig) error
```

`NotifyConfig` mirrors the SMTP fields from the daemon YAML config. Reads config file directly (does not require daemon to be running). Called only when `report.Failed > 0`.

Email format:
- **Subject:** `[homelab] Post-activation doctor: N check(s) failed`
- **Body:** plain-text report, one line per check with ✔/✗, detail, and fix hint for failures

---

## Section 3: NixOS Activation Integration

### New `module.nix` option

```nix
services.homelab-daemon.enableDoctorOnActivation = lib.mkOption {
  type = lib.types.bool;
  default = true;
  description = ''
    Run `homelab doctor` after each NixOS activation (nh os switch).
    Results are written to the journal (tag: homelab-doctor).
    Failures emit an SMTP notification but never block the switch.
  '';
};
```

### Layer 1: Activation script (synchronous, soft)

Runs during `nh os switch`, after mounts are up. Always exits 0 — the switch is never blocked.

```nix
system.activationScripts.homelabDoctor = lib.mkIf cfg.enableDoctorOnActivation {
  deps = [ "specialfs" ];
  text = ''
    ${pkgs.homelab-daemon}/bin/homelab doctor --json 2>&1 \
      | ${pkgs.systemd}/bin/systemd-cat -t homelab-doctor -p info || true
  '';
};
```

Results queryable with:
```bash
journalctl -t homelab-doctor --since "1 hour ago"
```

### Layer 2: Oneshot systemd service (async, persistent status)

Runs after activation. Status persists in `systemctl` between switches.

```nix
systemd.services.homelab-doctor-report = lib.mkIf cfg.enableDoctorOnActivation {
  description = "Homelab post-activation doctor report";
  wantedBy = [ "multi-user.target" ];
  after = [ "homelab-daemon.service" "network-online.target" ];
  serviceConfig = {
    Type = "oneshot";
    RemainAfterExit = true;
    ExecStart = "${pkgs.homelab-daemon}/bin/homelab doctor --json --fail-on-error";
    StandardOutput = "journal";
    StandardError = "journal";
    SyslogIdentifier = "homelab-doctor";
    # Soft warning: service reports failure state but does not cause cascading failures
    FailureAction = "none";
  };
};
```

Operator check after any switch:
```bash
systemctl status homelab-doctor-report
```

### Notification flow

The `homelab doctor --fail-on-error` path (Layer 2) exits 1 on failures. The service enters failed state. An `ExecStopPost` script or a companion `homelab-doctor-notify.service` (triggered via `OnFailure=`) calls:

```bash
homelab doctor --json | homelab-notify-doctor
```

where `homelab-notify-doctor` is a small subcommand added to the CLI:

```
homelab doctor notify   # reads JSON report from stdin, sends SMTP if failures present
```

This keeps the notification logic in Go (reuses `internal/doctor.Notify`) and avoids shell parsing of JSON.

---

## Files Changed

| File | Change |
|---|---|
| `cmd/cli/main.go` | Rewrite with urfave/cli v2, wire to internal/doctor |
| `internal/doctor/doctor.go` | New — check registry, Run(), Report types |
| `internal/doctor/notify.go` | New — Notify(), NotifyConfig, config reader |
| `module.nix` | Add `enableDoctorOnActivation` option, activation script, oneshot service |
| `go.mod` / `vendor/` | Add `github.com/urfave/cli/v2` |

---

## Out of Scope

- Moving doctor checks server-side (circular dependency: doctor checks whether daemon is running)
- NixOS VM test framework integration (overkill for a post-deploy smoke test)
- Dashboard URL config UI in frontend
