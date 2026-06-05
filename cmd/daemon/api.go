package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	logging "github.com/ogglord/homelab-logging"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ogglord/homelab-daemon/internal/cmdrunner"
	"github.com/ogglord/homelab-daemon/internal/collector"
	"github.com/ogglord/homelab-daemon/internal/storage/bcachefs"
	"github.com/ogglord/homelab-daemon/internal/updates"
	"github.com/ogglord/homelab-daemon/internal/vms"
	api "github.com/ogglord/homelab-api"
)

// computeBlockedReason returns a human-readable explanation for why
// the daemon is not starting/restarting a service. Returns empty string
// if the service is active or there is no clear blocker.
func computeBlockedReason(svc Service, activeState string, failureCount int, backoffUntil time.Time, userStopped bool) string {
	if activeState == "active" {
		return ""
	}
	if userStopped {
		return "stopped by user"
	}
	if err := checkConstraints(svc); err != nil {
		return err.Error()
	}
	if !backoffUntil.IsZero() && time.Now().Before(backoffUntil) {
		remaining := time.Until(backoffUntil).Round(time.Second)
		return fmt.Sprintf("backing off (%d failures), retry in %s", failureCount, remaining)
	}
	return ""
}

// serveAPI listens on a Unix socket and serves the daemon HTTP API.
//
// Routes:
//
//	GET  /api/status          — all managed services and their state
//	POST /api/start/:unit     — start a unit, clear user-stopped flag
//	POST /api/stop/:unit      — stop a unit, set user-stopped flag
//	POST /api/start-all       — start all enabled units
//	POST /api/stop-all        — stop all enabled units
//	POST /api/reload          — reload config from disk
//	GET  /api/config          — get static configuration for all services
//	PATCH /api/config/:unit   — update configuration for one service
//	GET  /api/backups         — list backups
//	POST /api/backups/:unit/run — manually trigger a backup
//	PATCH /api/backups/:unit  — update configuration for one backup
var apiLog = logging.Logger("api")

func serveAPI(ctx context.Context, sockPath string, cfg *Config, state *State, cfgPath string, breaker *CircuitBreaker, scheduler *Scheduler, updatesMod *updates.Module, col *collector.Collector) error {
	// Remove stale socket.
	_ = os.Remove(sockPath)

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		return err
	}
	// Allow homelab-dash (running as different user) to connect.
	if err := os.Chmod(sockPath, 0o660); err != nil {
		apiLog.Warn("chmod socket failed", "error", err)
	}
	// Wrap the listener so every accepted connection is queried via
	// SO_PEERCRED and its uid surfaced into per-request context. Used by
	// peerUIDGuard to enforce auth on destructive routes.
	pcl, pclOK := wrapListener(ln)
	if pclOK {
		ln = pcl
	}

	mux := http.NewServeMux()
	registerSecretsAPI(mux)

	// piProjectHandler returns the first pi-web project UUID so the dashboard
	// can link directly to sessions.cignl.cc/?project=<uuid>.
	piProjectHandler := func(w http.ResponseWriter, _ *http.Request) {
		resp, err := http.Get("http://127.0.0.1:8504/api/projects?cwd=/home/ogge")
		if err != nil {
			writeJSON(w, map[string]any{"projectId": "", "error": err.Error()})
			return
		}
		defer resp.Body.Close()
		var projects []struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
			writeJSON(w, map[string]any{"projectId": "", "error": err.Error()})
			return
		}
		id := ""
		if len(projects) > 0 {
			id = projects[0].ID
		}
		writeJSON(w, map[string]any{"projectId": id})
	}
	mux.HandleFunc("GET /api/pi/project", piProjectHandler)
	mux.HandleFunc("GET /api/v1/pi/project", piProjectHandler)

	// piStreamingHandler proxies pi-web sessiond's /health endpoint to
	// provide a streaming indicator for the Agent pill in the dashboard.
	piStreamingHandler := func(w http.ResponseWriter, _ *http.Request) {
		client := http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", "/cache/appdata/pi-web/sessiond.sock")
				},
			},
			Timeout: 3 * time.Second,
		}
		resp, err := client.Get("http://localhost/health")
		if err != nil {
			writeJSON(w, map[string]any{"streaming": false, "error": err.Error()})
			return
		}
		defer resp.Body.Close()
		var health struct {
			ActiveSessions int `json:"activeSessions"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
			writeJSON(w, map[string]any{"streaming": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{
			"streaming":      health.ActiveSessions > 0,
			"activeSessions": health.ActiveSessions,
		})
	}
	mux.HandleFunc("GET /api/pi/streaming", piStreamingHandler)
	mux.HandleFunc("GET /api/v1/pi/streaming", piStreamingHandler)

	statusHandler := func(w http.ResponseWriter, r *http.Request) {
		// Batch-fetch ActiveState, SubState, Description from systemd
		// so the dash doesn't need to probe systemctl directly.
		units := make([]string, 0, len(cfg.Services))
		for _, svc := range cfg.Services {
			units = append(units, svc.Unit)
		}
		unitProps := fetchSystemdProps(units, "Id", "ActiveState", "SubState", "Description")

		out := make([]api.ServiceStatus, 0, len(cfg.Services))
		for _, svc := range cfg.Services {
			consecutive, backoffUntil := breaker.State(svc.Unit)
			bu := ""
			if !backoffUntil.IsZero() && time.Now().Before(backoffUntil) {
				bu = backoffUntil.UTC().Format(time.RFC3339)
			}

			props := unitProps[svc.Unit]
			activeState := props["ActiveState"]
			subState := props["SubState"]
			desc := props["Description"]

			out = append(out, api.ServiceStatus{
				Unit:           svc.Unit,
				Enabled:        svc.Enabled,
				Active:         activeState == "active",
				ActiveState:    activeState,
				SubState:       subState,
				Description:    desc,
				UserStopped:    state.IsUserStopped(svc.Unit),
				Restart:        svc.Restart,
				Order:          svc.Order,
				BootDelay:      svc.BootDelay,
				RestartDelay:   svc.RestartDelay,
				DependsOn:      svc.DependsOn,
				RequiresMounts: svc.RequiresMounts,
				FailureCount:   consecutive,
				BackoffUntil:   bu,
				BlockedReason:  computeBlockedReason(svc, activeState, consecutive, backoffUntil, state.IsUserStopped(svc.Unit)),
			})
		}
		writeJSON(w, out)
	}
	mux.HandleFunc("GET /api/status", statusHandler)

	mux.HandleFunc("POST /api/start/{unit}", func(w http.ResponseWriter, r *http.Request) {
		unit := r.PathValue("unit")
		if !isManaged(cfg, unit) {
			http.Error(w, `{"error":"unknown unit"}`, http.StatusBadRequest)
			return
		}
		// Always clear user-stopped when user explicitly requests a start,
		// even if constraints block the actual systemd start. Otherwise the
		// blocked_reason would stay "stopped by user" and confuse the user.
		state.SetUserStopped(unit, false)

		// Enforce constraints (mounts) before starting.
		for _, svc := range cfg.Services {
			if svc.Unit == unit {
				if err := checkConstraints(svc); err != nil {
					apiLog.Warn("API start blocked by constraints", "unit", unit, "error", err)
					writeJSON(w, map[string]any{"success": false, "error": err.Error()})
					return
				}
				break
			}
		}
		if err := startUnit(unit); err != nil {
			apiLog.Error("API start failed", "unit", unit, "error", err)
			writeJSON(w, map[string]any{"success": false, "error": err.Error()})
			return
		}
		apiLog.Info("API start", "unit", unit)
		writeJSON(w, map[string]any{"success": true})
	})

	mux.HandleFunc("POST /api/stop/{unit}", func(w http.ResponseWriter, r *http.Request) {
		unit := r.PathValue("unit")
		if !isManaged(cfg, unit) {
			http.Error(w, `{"error":"unknown unit"}`, http.StatusBadRequest)
			return
		}
		state.SetUserStopped(unit, true)
		if err := stopUnit(unit); err != nil {
			apiLog.Error("API stop failed", "unit", unit, "error", err)
			writeJSON(w, map[string]any{"success": false, "error": err.Error()})
			return
		}
		apiLog.Info("API stop", "unit", unit)
		writeJSON(w, map[string]any{"success": true})
	})

	mux.HandleFunc("POST /api/restart/{unit}", func(w http.ResponseWriter, r *http.Request) {
		unit := r.PathValue("unit")
		if !isManaged(cfg, unit) {
			http.Error(w, `{"error":"unknown unit"}`, http.StatusBadRequest)
			return
		}
		state.SetUserStopped(unit, false)
		if err := restartUnit(unit); err != nil {
			apiLog.Error("API restart failed", "unit", unit, "error", err)
			writeJSON(w, map[string]any{"success": false, "error": err.Error()})
			return
		}
		apiLog.Info("API restart", "unit", unit)
		// Trigger an update check after restarting a podman container
		// (the image may have changed on restart).
		if strings.HasPrefix(unit, "podman-") {
			go updatesMod.TriggerCheck()
		}
		writeJSON(w, map[string]any{"success": true})
	})

	mux.HandleFunc("POST /api/start-all", func(w http.ResponseWriter, r *http.Request) {
		units := enabledUnits(cfg)
		state.SetAllUserStopped(units, false)
		failed := 0
		for _, u := range units {
			// Enforce constraints before starting.
			blocked := false
			for _, svc := range cfg.Services {
				if svc.Unit == u {
					if err := checkConstraints(svc); err != nil {
						apiLog.Warn("start-all blocked by constraints", "unit", u, "error", err)
						failed++
						blocked = true
					}
					break
				}
			}
			if blocked {
				continue
			}
			if err := startUnit(u); err != nil {
				apiLog.Error("start-all failed for unit", "unit", u, "error", err)
				failed++
			}
		}
		apiLog.Info("API start-all", "total", len(units), "failed", failed)
		writeJSON(w, map[string]any{"success": failed == 0, "total": len(units), "failed": failed})
	})

	mux.HandleFunc("POST /api/stop-all", func(w http.ResponseWriter, r *http.Request) {
		units := enabledUnits(cfg)
		state.SetAllUserStopped(units, true)
		failed := 0
		for _, u := range units {
			if err := stopUnit(u); err != nil {
				apiLog.Error("stop-all failed for unit", "unit", u, "error", err)
				failed++
			}
		}
		apiLog.Info("API stop-all", "total", len(units), "failed", failed)
		writeJSON(w, map[string]any{"success": failed == 0, "total": len(units), "failed": failed})
	})

	mux.HandleFunc("POST /api/reload", func(w http.ResponseWriter, r *http.Request) {
		newCfg, err := loadConfig(cfgPath)
		if err != nil {
			apiLog.Error("reload failed", "error", err)
			writeJSON(w, map[string]any{"success": false, "error": err.Error()})
			return
		}
		*cfg = *newCfg
		scheduler.Reload(cfg)
		apiLog.Info("config reloaded", "services", len(cfg.Services))
		// Re-verify constraints against running services.
		if n := verifyRunningServices(cfg); n > 0 {
			apiLog.Info("constraint verification stopped running services", "count", n)
		}
		writeJSON(w, map[string]any{"success": true, "services": len(cfg.Services)})
	})

	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, cfg.Services)
	})

	mux.HandleFunc("PATCH /api/config/{unit}", func(w http.ResponseWriter, r *http.Request) {
		unit := r.PathValue("unit")
		if !strings.HasSuffix(unit, ".service") {
			unit += ".service"
		}

		var payload struct {
			Enabled      *bool     `json:"enabled"`
			Order        *int      `json:"order"`
			BootDelay    *int      `json:"boot_delay"`
			DependsOn    *[]string `json:"depends_on"`
			Restart      *string   `json:"restart"`
			RestartDelay *int      `json:"restart_delay"`
		}

		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}

		// Find service
		idx := -1
		for i, svc := range cfg.Services {
			if svc.Unit == unit {
				idx = i
				break
			}
		}

		if idx == -1 {
			http.Error(w, `{"error":"unknown unit"}`, http.StatusNotFound)
			return
		}

		// Apply changes
		if payload.Enabled != nil {
			cfg.Services[idx].Enabled = *payload.Enabled
		}
		if payload.Order != nil {
			cfg.Services[idx].Order = *payload.Order
		}
		if payload.BootDelay != nil {
			cfg.Services[idx].BootDelay = *payload.BootDelay
		}
		if payload.DependsOn != nil {
			cfg.Services[idx].DependsOn = *payload.DependsOn
		}
		if payload.Restart != nil {
			restartVal := *payload.Restart
			switch restartVal {
			case "no", "on-failure", "unless-stopped", "always":
				cfg.Services[idx].Restart = restartVal
			default:
				http.Error(w, `{"error":"invalid restart policy"}`, http.StatusBadRequest)
				return
			}
		}
		if payload.RestartDelay != nil {
			cfg.Services[idx].RestartDelay = *payload.RestartDelay
		}

		// Write config back to disk atomically
		if err := writeConfig(cfgPath, cfg); err != nil {
			apiLog.Error("failed to write config", "error", err)
			http.Error(w, `{"error":"failed to save config"}`, http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]any{"success": true, "service": cfg.Services[idx]})
	})

	mux.HandleFunc("GET /api/backups", func(w http.ResponseWriter, r *http.Request) {
		out := make([]api.BackupStatus, 0, len(cfg.Backups))
		for _, b := range cfg.Backups {
			bs := api.BackupStatus{Backup: api.Backup{
				Unit:            b.Unit,
				Enabled:         b.Enabled,
				Schedule:        b.Schedule,
				DependsOn:       b.DependsOn,
				RequiresMounts:  b.RequiresMounts,
				HealthcheckUUID: b.HealthcheckUUID,
				PauseService:    b.PauseService,
			}}
			// Fetch status using systemctl show
			cmdRes, err := cmdrunner.New("api", "systemctl", "show", b.Unit, "--property=ActiveState,Result,ExecMainStartTimestamp,ExecMainExitTimestamp").
				WithContext(r.Context()).
				Run()
			if err == nil {
				m := make(map[string]string)
				for _, line := range strings.Split(cmdRes.Stdout, "\n") {
					if k, v, ok := strings.Cut(line, "="); ok {
						m[k] = v
					}
				}
				bs.ActiveState = m["ActiveState"]
				bs.Result = m["Result"]
				// Parse systemd timestamp → RFC3339 so the frontend can display it.
				// Format: "Mon 2006-01-02 15:04:05 MST"
				if ts := m["ExecMainStartTimestamp"]; ts != "" {
					if t, err := time.Parse("Mon 2006-01-02 15:04:05 MST", ts); err == nil {
						bs.LastRunStart = t.Format(time.RFC3339)
					}
				}
				if ts := m["ExecMainExitTimestamp"]; ts != "" {
					bs.LastRunEnd = ts
				}
			}

			// Override times with cron scheduler info since systemctl oneshot drops timestamps
			if entry := scheduler.GetEntry(b.Unit); entry != nil {
				if !entry.Prev.IsZero() {
					bs.LastRunStart = entry.Prev.Format(time.RFC3339)
				}
				bs.NextRun = entry.Next.Format(time.RFC3339)
			}

			// Fall back to the scheduler's own last-run record (survives daemon restarts for
			// the lifetime of this process, at least).
			if bs.LastRunStart == "" {
				if lr := scheduler.LastRunStart(b.Unit); !lr.IsZero() {
					bs.LastRunStart = lr.Format(time.RFC3339)
				}
			}

			out = append(out, bs)
		}
		writeJSON(w, out)
	})

	mux.HandleFunc("POST /api/backups/{unit}/run", func(w http.ResponseWriter, r *http.Request) {
		unit := r.PathValue("unit")
		if !strings.HasSuffix(unit, ".service") {
			unit += ".service"
		}
		go func() {
			if err := scheduler.RunBackup(unit); err != nil {
				apiLog.Error("manual backup run failed", "unit", unit, "error", err)
			}
		}()
		writeJSON(w, map[string]any{"success": true})
	})

	mux.HandleFunc("GET /api/backups/{unit}/logs", func(w http.ResponseWriter, r *http.Request) {
		unit := r.PathValue("unit")
		if !strings.HasSuffix(unit, ".service") {
			unit += ".service"
		}
		res, err := cmdrunner.New("api", "journalctl", "-u", unit, "-n", "50", "--no-pager", "-l").
			WithContext(r.Context()).
			Output(cmdrunner.OutputCombined).
			Run()
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(res.Output))
	})

	mux.HandleFunc("PATCH /api/backups/{unit}", func(w http.ResponseWriter, r *http.Request) {
		unit := r.PathValue("unit")
		if !strings.HasSuffix(unit, ".service") {
			unit += ".service"
		}

		var payload struct {
			Enabled         *bool     `json:"enabled"`
			Schedule        *string   `json:"schedule"`
			HealthcheckUUID *string   `json:"healthcheck_uuid"`
			PauseService    *string   `json:"pause_service"`
			DependsOn       *[]string `json:"depends_on"`
		}

		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}

		idx := -1
		for i, b := range cfg.Backups {
			if b.Unit == unit {
				idx = i
				break
			}
		}

		if idx == -1 {
			http.Error(w, `{"error":"backup not found"}`, http.StatusNotFound)
			return
		}

		if payload.Enabled != nil {
			cfg.Backups[idx].Enabled = *payload.Enabled
		}
		if payload.Schedule != nil {
			cfg.Backups[idx].Schedule = *payload.Schedule
		}
		if payload.HealthcheckUUID != nil {
			cfg.Backups[idx].HealthcheckUUID = *payload.HealthcheckUUID
		}
		if payload.PauseService != nil {
			cfg.Backups[idx].PauseService = *payload.PauseService
		}
		if payload.DependsOn != nil {
			cfg.Backups[idx].DependsOn = *payload.DependsOn
		}

		if err := writeConfig(cfgPath, cfg); err != nil {
			apiLog.Error("failed to write config", "error", err)
			http.Error(w, `{"error":"failed to save config"}`, http.StatusInternalServerError)
			return
		}

		scheduler.Reload(cfg)
		writeJSON(w, map[string]any{"success": true, "backup": cfg.Backups[idx]})
	})

	// Storage APIs
	// storageCache keeps /api/storage responses fresh for a short TTL so the
	// client's 2-3s poll interval doesn't hammer disk-level bcachefs CLI calls.
	type storageCacheEntry struct {
		json     []byte // pre-serialized JSON response
		deadline time.Time
	}
	var (
		storageCache     *storageCacheEntry
		storageCacheMu   sync.Mutex
		storageCacheTTL  = 10 * time.Second
	)
	mux.HandleFunc("GET /api/storage", func(w http.ResponseWriter, r *http.Request) {
		storageCacheMu.Lock()
		if storageCache != nil && time.Now().Before(storageCache.deadline) {
			data := storageCache.json
			storageCacheMu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
			return
		}
		storageCacheMu.Unlock()

		pools, err := bcachefs.DiscoverPools()
		if err != nil {
			apiLog.Error("DiscoverPools failed", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Attach names and pool-level usage
		for i := range pools {
			p := &pools[i]
			resolvedName := ""
			for _, cp := range cfg.Storage.Pools {
				if cp.UUID == p.UUID && cp.Name != "" {
					resolvedName = cp.Name
					break
				}
			}
			if resolvedName == "" {
				if p.Label != "" {
					resolvedName = p.Label
				} else {
					if len(p.UUID) >= 8 {
						resolvedName = "pool-" + p.UUID[:8]
					} else {
						resolvedName = "pool-" + p.UUID
					}
				}
			}
			p.Name = resolvedName

			if p.State == "mounted" && p.Mountdir != "" {
				usage, err := bcachefs.GetPoolUsage(p.Mountdir)
				if err == nil {
					p.Usage = usage
				} else {
					apiLog.Error("failed to get pool usage", "mountdir", p.Mountdir, "error", err)
				}
			}
		}

		// Get disk I/O stats
		var diskNames []string
		for _, p := range pools {
			for _, d := range p.Disks {
				if d.Name != "" {
					diskNames = append(diskNames, d.Name)
				}
			}
		}
		if len(diskNames) > 0 {
			ioStats, err := bcachefs.GetDiskIO(diskNames)
			if err == nil {
				for i := range pools {
					for j := range pools[i].Disks {
						d := &pools[i].Disks[j]
						if stats, ok := ioStats[d.Name]; ok {
							d.IO = &stats
						}
					}
				}
			} else {
				apiLog.Error("failed to get disk I/O stats", "error", err)
			}
		}

		unassigned, err := bcachefs.GetUnassignedDisks()
		if err != nil {
			apiLog.Error("GetUnassignedDisks failed", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var subvolumes []bcachefs.Subvolume
		for _, p := range pools {
			if p.State == "mounted" && p.Mountdir != "" {
				subs, err := bcachefs.ListSubvolumes(p.Mountdir)
				if err == nil {
					subvolumes = append(subvolumes, subs...)
				}
			}
		}

		resp := map[string]any{
			"pools":      pools,
			"unassigned": unassigned,
			"subvolumes": subvolumes,
		}

		raw, err := json.Marshal(resp)
		if err != nil {
			apiLog.Error("storage marshal failed", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// Invalidate the stuck / stale bit so the next poll that drifts past
		// TTL still picks up something fresh-ish, not a relic.
		storageCacheMu.Lock()
		storageCache = &storageCacheEntry{json: raw, deadline: time.Now().Add(storageCacheTTL)}
		storageCacheMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.Write(raw)
	})

	mux.HandleFunc("POST /api/storage/mount", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Devices    []string `json:"devices"`
			Mountpoint string   `json:"mountpoint"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		if err := bcachefs.Mount(payload.Devices, payload.Mountpoint); err != nil {
			apiLog.Error("Mount failed", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"success": true})
	})

	mux.HandleFunc("POST /api/storage/unmount", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Mountpoint string `json:"mountpoint"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}

		var activeDeps []string
		for _, svc := range cfg.Services {
			for _, mnt := range svc.RequiresMounts {
				if mnt == payload.Mountpoint {
					if isActive(svc.Unit) {
						activeDeps = append(activeDeps, svc.Unit)
					}
					break
				}
			}
		}
		if len(activeDeps) > 0 {
			apiLog.Warn("Safe unmount blocked", "mountpoint", payload.Mountpoint, "active_deps", activeDeps)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]any{
				"error":       "dependent_services_running",
				"message":     fmt.Sprintf("Cannot unmount %s: %d dependent services are running", payload.Mountpoint, len(activeDeps)),
				"active_deps": activeDeps,
			})
			return
		}

		if err := bcachefs.Unmount(payload.Mountpoint); err != nil {
			apiLog.Error("Unmount failed", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"success": true})
	})

	mux.HandleFunc("POST /api/storage/subvolume-usage", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Paths []string `json:"paths"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}

		usageMap := make(map[string]int64)
		for _, path := range payload.Paths {
			size, err := bcachefs.GetSubvolumeUsage(path)
			if err != nil {
				apiLog.Error("failed to get subvolume usage", "path", path, "error", err)
				usageMap[path] = -1
			} else {
				usageMap[path] = size
			}
		}

		writeJSON(w, map[string]any{"success": true, "usage": usageMap})
	})

	mux.HandleFunc("PATCH /api/storage/config", func(w http.ResponseWriter, r *http.Request) {
		var payload api.StorageConfigPatch
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}

		// Translate api.StoragePoolConfig → internal StoragePoolConfig.
		// Identical fields (same JSON/YAML tags); copy element-wise to
		// keep the YAML writer happy.
		pools := make([]StoragePoolConfig, len(payload.Pools))
		for i, p := range payload.Pools {
			pools[i] = StoragePoolConfig{
				UUID:       p.UUID,
				Mountpoint: p.Mountpoint,
				Options:    p.Options,
				AutoMount:  p.AutoMount,
				Name:       p.Name,
			}
		}
		cfg.Storage.Pools = pools
		if err := writeConfig(cfgPath, cfg); err != nil {
			apiLog.Error("failed to write config", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		apiLog.Info("storage config updated via API")
		writeJSON(w, map[string]any{"success": true})
	})

	mux.HandleFunc("POST /api/storage/subvolume", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		if err := bcachefs.CreateSubvolume(payload.Path); err != nil {
			apiLog.Error("CreateSubvolume failed", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"success": true})
	})

	mux.HandleFunc("DELETE /api/storage/subvolume", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		if path == "" {
			http.Error(w, `{"error":"path query parameter required"}`, http.StatusBadRequest)
			return
		}
		if err := bcachefs.DeleteSubvolume(path); err != nil {
			apiLog.Error("DeleteSubvolume failed", "error", err)
			if errors.Is(err, bcachefs.ErrSubvolumeNotEmpty) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(map[string]any{
					"error": err.Error(),
					"code":  "subvolume_not_empty",
				})
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"success": true})
	})

	mux.HandleFunc("POST /api/storage/init-folders", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Mountpoint string `json:"mountpoint"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		if err := bcachefs.InitFolders(payload.Mountpoint); err != nil {
			apiLog.Error("InitFolders failed", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"success": true})
	})

	// ── Libvirt VM Query and Actions ─────────────────────────────────────
	mux.HandleFunc("GET /api/vms", func(w http.ResponseWriter, r *http.Request) {
		vmMod := vms.New("")
		defer vmMod.Close()

		list, err := vmMod.GetVMs()
		if err != nil {
			apiLog.Error("Failed to list VMs", "error", err)
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}
		writeJSON(w, list)
	})

	mux.HandleFunc("POST /api/vms/{name}/{action}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		action := r.PathValue("action")

		vmMod := vms.New("")
		defer vmMod.Close()

		if err := vmMod.RunAction(name, action); err != nil {
			apiLog.Error("Failed to run VM action", "name", name, "action", action, "error", err)
			writeJSON(w, map[string]any{"success": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"success": true})
	})

	// ── Unified Logs Endpoint (Journalctl or Podman) ─────────────────────
	mux.HandleFunc("GET /api/services/{unit}/logs", func(w http.ResponseWriter, r *http.Request) {
		unit := r.PathValue("unit")
		isPodman := strings.HasPrefix(unit, "podman-")

		if isPodman {
			name := strings.TrimSuffix(strings.TrimPrefix(unit, "podman-"), ".service")
			res, _ := cmdrunner.New("api", "podman", "logs", name, "--tail", "50").
				WithContext(r.Context()).
				Output(cmdrunner.OutputCombined).
				Run()

			writeJSON(w, map[string]any{"success": true, "logs": res.Output})
			return
		}

		res, _ := cmdrunner.New("api", "journalctl", "-u", unit, "-n", "50", "--no-pager", "-l").
			WithContext(r.Context()).
			Output(cmdrunner.OutputCombined).
			Run()

		writeJSON(w, map[string]any{"success": true, "logs": res.Output})
	})

	// ── Streaming Container Image Pull SSE Endpoint ──────────────────────
	mux.HandleFunc("GET /api/services/{unit}/pull-stream", func(w http.ResponseWriter, r *http.Request) {
		unit := r.PathValue("unit")
		image := ""

		metadata := updatesMod.GetMetadata()
		name := strings.TrimSuffix(strings.TrimPrefix(unit, "podman-"), ".service")
		if entry, ok := metadata[name]; ok {
			image = entry.Image
		}

		if image == "" {
			http.Error(w, `{"error":"service not found or no image info available"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Transfer-Encoding", "chunked")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		_, _ = w.Write([]byte("event: connected\ndata: \n\n"))
		flusher.Flush()

		_, err := cmdrunner.New("api", "podman", "pull", "--policy", "newer", "--platform", "linux/amd64", image).
			WithContext(r.Context()).
			WithLineHandler(func(stream, line string) {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" {
					_, _ = w.Write([]byte(fmt.Sprintf("event: log\ndata: %s\n\n", trimmed)))
					flusher.Flush()
				}
			}).
			Run()

		if err != nil {
			_, _ = w.Write([]byte("event: pull-error\ndata: pull failed: " + err.Error() + "\n\n"))
		} else {
			_, _ = w.Write([]byte("event: done\ndata: pull complete\n\n"))
			updatesMod.TriggerCheck()
		}
		flusher.Flush()
	})

	// ── Podman Container Updates Registry Query/Trigger ──────────────────
	mux.HandleFunc("GET /api/updates", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"updates":  updatesMod.GetUpdates(),
			"metadata": updatesMod.GetMetadata(),
		})
	})

	mux.HandleFunc("POST /api/updates/check", func(w http.ResponseWriter, r *http.Request) {
		updatesMod.TriggerCheck()
		writeJSON(w, map[string]any{"success": true})
	})

	// ── Merged Services Endpoint ─────────────────────────────────────────
	// Returns a superset of all fields the frontend consumes, joining
	// systemd state with update metadata and config. The dash calls this
	// instead of assembling the same shape locally.
	mux.HandleFunc("GET /api/services/merged", func(w http.ResponseWriter, r *http.Request) {
		units := make([]string, 0, len(cfg.Services))
		for _, svc := range cfg.Services {
			units = append(units, svc.Unit)
		}
		unitProps := fetchSystemdProps(units, "Id", "ActiveState", "SubState", "Description")
		updates := updatesMod.GetUpdates()
		mdata := updatesMod.GetMetadata()

		out := make([]api.ServiceInfo, 0, len(cfg.Services))
		for _, svc := range cfg.Services {
			u := svc.Unit
			name := strings.TrimPrefix(u, "podman-")
			name = strings.TrimSuffix(name, ".service")
			name = strings.TrimSuffix(name, ".socket")

			props := unitProps[u]
			activeState := props["ActiveState"]
			subState := props["SubState"]
			desc := props["Description"]

			image := ""
			isDocker := strings.HasPrefix(u, "podman-")
			if isDocker {
				if entry, ok := mdata[name]; ok {
					desc = entry.Description
					image = entry.Image
				}
			}

			svcType := "Native"
			if isDocker {
				svcType = "Docker"
			}

			up := updates[name]

			consecutive, backoffUntil := breaker.State(u)
			backoffSecs := 0
			if !backoffUntil.IsZero() && time.Now().Before(backoffUntil) {
				backoffSecs = int(time.Until(backoffUntil).Seconds())
			}

			out = append(out, api.ServiceInfo{
				Name:            name,
				Type:            svcType,
				Image:           image,
				ActiveState:     or(activeState, "inactive"),
				SubState:        or(subState, "dead"),
				Description:     desc,
				UnitName:        u,
				UpdateAvailable: up.HasUpdate,
				CurrentVersion:  or(up.CurrentVersion, up.LocalID),
				RemoteVersion:   or(up.RemoteVersion, up.RemoteID),
				DaemonManaged:   true,
				RestartPolicy:   svc.Restart,
				BootOrder:       svc.Order,
				BootDelay:       svc.BootDelay,
				RestartDelay:    svc.RestartDelay,
				DependsOn:       svc.DependsOn,
				UserStopped:     state.IsUserStopped(u),
				DaemonEnabled:   svc.Enabled,
				FailureCount:    consecutive,
				BackoffSeconds:  backoffSecs,
			})
		}

		sort.Slice(out, func(i, j int) bool {
			if out[i].Type != out[j].Type {
				return out[i].Type < out[j].Type
			}
			return out[i].Name < out[j].Name
		})

		writeJSON(w, out)
	})

	// ── Diagnostics Endpoint (Critical Infra Services) ───────────────────
	// Returns status for low-level infrastructure units that are *not*
	// daemon-managed but are vital for the dashboard to display.
	mux.HandleFunc("GET /api/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		infraUnits := []string{
			"homelab-daemon.service",
			"tailscaled.service",
			"libvirtd.service",
			"podman.socket",
			"caddy.service",
			"victorialogs.service",
			"victorialogs-mcp.service",
		}
		unitProps := fetchSystemdProps(infraUnits, "Id", "ActiveState", "SubState", "Description")

		out := make([]api.ServiceInfo, 0, len(infraUnits))
		for _, u := range infraUnits {
			props := unitProps[u]
			name := strings.TrimSuffix(u, ".service")
			name = strings.TrimSuffix(name, ".socket")

			out = append(out, api.ServiceInfo{
				Name:          name,
				Type:          "Native",
				ActiveState:   or(props["ActiveState"], "inactive"),
				SubState:      or(props["SubState"], "dead"),
				Description:   props["Description"],
				UnitName:      u,
				DaemonManaged: false,
			})
		}

		sort.Slice(out, func(i, j int) bool {
			return out[i].Name < out[j].Name
		})

		writeJSON(w, out)
	})

	// ── Host Stats Endpoint ──────────────────────────────────────────────
	// Returns the latest collector snapshot. The dash replaces its own
	// gopsutil-based poller with this endpoint.
	mux.HandleFunc("GET /api/stats", func(w http.ResponseWriter, r *http.Request) {
		if col == nil {
			http.Error(w, `{"error":"collector not initialized"}`, http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, col.Get())
	})

	// ── Wire-contract version probe ──────────────────────────────────────
	// Unversioned (/api/version) so clients can ask "which API do you
	// speak?" before picking a prefix. Same body served at
	// /api/v1/version for symmetry once the v1 mux is mounted.
	mux.HandleFunc("GET /api/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, api.VersionResponse{
			Version:    api.Version,
			APIVersion: api.Version,
			Prefix:     api.APIPrefix,
		})
	})

	// ── Aggregated Overview Endpoint ────────────────────────────────────────
	// Returns stats + services + VMs + backups in a single response so the
	// frontend needs only one poll call per tick.
	mux.HandleFunc("GET /api/overview", func(w http.ResponseWriter, r *http.Request) {
		hostname, _ := os.Hostname()

		// Stats from in-memory collector (fast, already cached).
		// Convert the internal collector.Stats -> api.StatsSnapshot via JSON
		// since they are separate Go types with identical fields and tags.
		var stats api.StatsSnapshot
		if col != nil {
			raw := col.Get()
			b, _ := json.Marshal(raw)
			json.Unmarshal(b, &stats)
		}

		// Services — same logic as /api/services/merged.
		units := make([]string, 0, len(cfg.Services))
		for _, svc := range cfg.Services {
			units = append(units, svc.Unit)
		}
		unitProps := fetchSystemdProps(units, "Id", "ActiveState", "SubState", "Description")
		updates := updatesMod.GetUpdates()
		mdata := updatesMod.GetMetadata()

		services := make([]api.ServiceInfo, 0, len(cfg.Services))
		for _, svc := range cfg.Services {
			u := svc.Unit
			name := strings.TrimPrefix(u, "podman-")
			name = strings.TrimSuffix(name, ".service")
			name = strings.TrimSuffix(name, ".socket")

			props := unitProps[u]
			activeState := props["ActiveState"]
			subState := props["SubState"]
			desc := props["Description"]

			image := ""
			isDocker := strings.HasPrefix(u, "podman-")
			if isDocker {
				if entry, ok := mdata[name]; ok {
					desc = entry.Description
					image = entry.Image
				}
			}

			svcType := "Native"
			if isDocker {
				svcType = "Docker"
			}

			up := updates[name]

			consecutive, backoffUntil := breaker.State(u)
			backoffSecs := 0
			if !backoffUntil.IsZero() && time.Now().Before(backoffUntil) {
				backoffSecs = int(time.Until(backoffUntil).Seconds())
			}

			services = append(services, api.ServiceInfo{
				Name:            name,
				Type:            svcType,
				Image:           image,
				ActiveState:     or(activeState, "inactive"),
				SubState:        or(subState, "dead"),
				Description:     desc,
				UnitName:        u,
				UpdateAvailable: up.HasUpdate,
				CurrentVersion:  or(up.CurrentVersion, up.LocalID),
				RemoteVersion:   or(up.RemoteVersion, up.RemoteID),
				DaemonManaged:   true,
				RestartPolicy:   svc.Restart,
				BootOrder:       svc.Order,
				BootDelay:       svc.BootDelay,
				RestartDelay:    svc.RestartDelay,
				DependsOn:       svc.DependsOn,
				UserStopped:     state.IsUserStopped(u),
				DaemonEnabled:   svc.Enabled,
				FailureCount:    consecutive,
				BackoffSeconds:  backoffSecs,
				BlockedReason:   computeBlockedReason(svc, activeState, consecutive, backoffUntil, state.IsUserStopped(u)),
				RequiresMounts:  svc.RequiresMounts,
			})
		}

		sort.Slice(services, func(i, j int) bool {
			if services[i].Type != services[j].Type {
				return services[i].Type < services[j].Type
			}
			return services[i].Name < services[j].Name
		})

		// VMs — fresh connect per request (libvirt unix socket is fast).
		// Convert internal vms.VMInfo -> api.VMInfo via JSON.
		vmsList := []api.VMInfo{}
		vmMod := vms.New("")
		if internalVMs, vmErr := vmMod.GetVMs(); vmErr == nil {
			b, _ := json.Marshal(internalVMs)
			json.Unmarshal(b, &vmsList)
		} else {
			apiLog.Warn("overview: failed to list VMs", "error", vmErr)
		}
		vmMod.Close()

		// Backups — batched systemctl show (single call for all units).
		backupUnits := make([]string, 0, len(cfg.Backups))
		for _, b := range cfg.Backups {
			backupUnits = append(backupUnits, b.Unit)
		}
		backupProps := fetchSystemdProps(backupUnits,
			"ActiveState", "Result", "ExecMainStartTimestamp", "ExecMainExitTimestamp",
		)

		backups := make([]api.BackupStatus, 0, len(cfg.Backups))
		for _, b := range cfg.Backups {
			bs := api.BackupStatus{Backup: api.Backup{
				Unit:            b.Unit,
				Enabled:         b.Enabled,
				Schedule:        b.Schedule,
				DependsOn:       b.DependsOn,
				RequiresMounts:  b.RequiresMounts,
				HealthcheckUUID: b.HealthcheckUUID,
				PauseService:    b.PauseService,
			}}
			if m := backupProps[b.Unit]; m != nil {
				bs.ActiveState = m["ActiveState"]
				bs.Result = m["Result"]
				if ts := m["ExecMainStartTimestamp"]; ts != "" {
					if t, err := time.Parse("Mon 2006-01-02 15:04:05 MST", ts); err == nil {
						bs.LastRunStart = t.Format(time.RFC3339)
					}
				}
				if ts := m["ExecMainExitTimestamp"]; ts != "" {
					bs.LastRunEnd = ts
				}
			}
			if entry := scheduler.GetEntry(b.Unit); entry != nil {
				if !entry.Prev.IsZero() {
					bs.LastRunStart = entry.Prev.Format(time.RFC3339)
				}
				bs.NextRun = entry.Next.Format(time.RFC3339)
			}
			if bs.LastRunStart == "" {
				if lr := scheduler.LastRunStart(b.Unit); !lr.IsZero() {
					bs.LastRunStart = lr.Format(time.RFC3339)
				}
			}
			backups = append(backups, bs)
		}

		writeJSON(w, struct {
			Hostname string              `json:"Hostname"`
			Stats    api.StatsSnapshot   `json:"Stats"`
			Services []api.ServiceInfo   `json:"Services"`
			VMs      []api.VMInfo        `json:"VMs"`
			Backups  []api.BackupStatus  `json:"Backups"`
		}{
			Hostname: hostname,
			Stats:    stats,
			Services: services,
			VMs:      vmsList,
			Backups:  backups,
		})
	})

	// ── /api/v1/* mirror ─────────────────────────────────────────────────
	// All handlers above are registered on /api/...; we expose the same
	// surface at /api/v1/... by stripping the /v1 segment and re-entering
	// the mux. New clients (and the dash, after this commit) should
	// prefer /api/v1/...; legacy /api/... stays alive for the transition.
	rootMux := http.NewServeMux()
	rootMux.Handle("/api/v1/", http.StripPrefix("/api/v1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/api" + r.URL.Path
		if r.URL.RawPath != "" {
			r2.URL.RawPath = "/api" + r.URL.RawPath
		}
		mux.ServeHTTP(w, r2)
	})))
	rootMux.Handle("/", mux)

	// WriteTimeout left at 0 because the /pull-stream SSE handler streams
	// for the duration of a `podman pull` (minutes on large images) and we
	// have no per-route timeout scoping today. ReadHeaderTimeout and
	// IdleTimeout still protect against slow-header / idle-keepalive abuse.
	// Wrap the root mux in cross-cutting middleware: request-id +
	// structured logging + Prometheus counters first, then SO_PEERCRED
	// enforcement on destructive routes.
	var handler http.Handler = rootMux
	handler = peerUIDGuard(handler)
	handler = requestIDLoggerMiddleware(handler)

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       120 * time.Second,
	}
	if pclOK {
		srv.ConnContext = pcl.connContext
	}

	// Graceful shutdown: when the parent context is cancelled, give in-flight
	// requests up to 10s to drain before closing the listener.
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			apiLog.Warn("API server shutdown error", "error", err)
		}
	}()

	apiLog.Info("API listening", "socket", sockPath)
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func isManaged(cfg *Config, unit string) bool {
	// Normalise: allow bare names like "immich-server" → "immich-server.service"
	if !strings.HasSuffix(unit, ".service") {
		unit += ".service"
	}
	for _, svc := range cfg.Services {
		if svc.Unit == unit {
			return true
		}
	}
	return false
}

func enabledUnits(cfg *Config) []string {
	var out []string
	for _, svc := range cfg.Services {
		if svc.Enabled {
			out = append(out, svc.Unit)
		}
	}
	return out
}

// fetchSystemdProps batch-fetches systemd properties for one or more units
// via a single `systemctl show` call. Returns map[unitName]map[property]value.
func fetchSystemdProps(units []string, props ...string) map[string]map[string]string {
	if len(units) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := []string{"show"}
	args = append(args, units...)
	args = append(args, "--property="+strings.Join(props, ","))

	cmdRes, err := cmdrunner.New("api", "systemctl", args...).WithContext(ctx).Run()
	if err != nil {
		return nil
	}

	result := make(map[string]map[string]string, len(units))
	// systemctl show multi-unit output: blank-line-separated sections,
	// each starting with "Id=<unit>".
	sections := strings.Split(strings.TrimSpace(cmdRes.Stdout), "\n\n")
	for _, section := range sections {
		if section == "" {
			continue
		}
		m := make(map[string]string)
		var id string
		for _, line := range strings.Split(section, "\n") {
			if k, v, ok := strings.Cut(line, "="); ok {
				m[k] = v
				if k == "Id" {
					id = v
				}
			}
		}
		if id != "" {
			result[id] = m
		}
	}
	return result
}

func or(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
