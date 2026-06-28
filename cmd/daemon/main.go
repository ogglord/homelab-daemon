// Package main — homelab-daemon entry point.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	logging "github.com/ogglord/homelab-logging"

	"github.com/ogglord/homelab-daemon/internal/cmdrunner"
	"github.com/ogglord/homelab-daemon/internal/collector"
	"github.com/ogglord/homelab-daemon/internal/notifier"
	"github.com/ogglord/homelab-daemon/internal/storage/bcachefs"
	"github.com/ogglord/homelab-daemon/internal/updates"
	"github.com/ogglord/homelab-daemon/internal/vpn"
	"golang.org/x/sync/errgroup"
)

var mainLog = logging.Logger("api")

func main() {
	logging.Init()

	if len(os.Args) > 1 && os.Args[1] == "merge-config" {
		handleMergeConfig()
		os.Exit(0)
	}

	// ── Hostname (for notification subjects) ────────────────────────────
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "homelab"
	}

	configPath := flag.String("config", "/cache/appdata/homelab/services.yaml", "path to services.yaml")
	registryPath := flag.String("registry", "/etc/homelab-daemon/managed-units", "path to NixOS-managed unit registry")
	stateDirFlag := flag.String("state-dir", "/var/lib/homelab-daemon", "directory for persistent daemon state (state.json, podman-metadata.json)")
	flag.Parse()

	// Startup health check
	mainLog.Info("starting homelab-daemon health check...")
	var hcErrors []string

	// Check if state directory is writable (or create it and verify)
	if err := os.MkdirAll(*stateDirFlag, 0o750); err != nil {
		hcErrors = append(hcErrors, fmt.Sprintf("state directory %q not writable: %v", *stateDirFlag, err))
	} else {
		// Verify we can write a test file
		testFile := *stateDirFlag + "/.healthcheck"
		if err := os.WriteFile(testFile, []byte("ok"), 0o600); err != nil {
			hcErrors = append(hcErrors, fmt.Sprintf("failed to write to state directory %q: %v", *stateDirFlag, err))
		} else {
			_ = os.Remove(testFile)
			mainLog.Info("state directory health check: healthy and writable", "path", *stateDirFlag)
		}
	}

	// Check if config file is readable
	if _, err := os.Stat(*configPath); err != nil {
		if os.IsNotExist(err) {
			mainLog.Warn("config file services.yaml does not exist, will bootstrap from defaults", "path", *configPath)
		} else {
			hcErrors = append(hcErrors, fmt.Sprintf("cannot read config path %q: %v", *configPath, err))
		}
	} else {
		mainLog.Info("config file health check: healthy and readable", "path", *configPath)
	}

	// Verify lookups of critical system dependencies (systemctl, podman, etc.)
	for _, tool := range []string{"systemctl", "podman"} {
		if _, err := exec.LookPath(tool); err != nil {
			mainLog.Warn("system tool dependency check warn", "tool", tool, "error", err.Error())
		} else {
			mainLog.Info("system tool dependency check: healthy", "tool", tool)
		}
	}

	if len(hcErrors) > 0 {
		mainLog.Warn("startup health check completed with warnings/errors", "errors", hcErrors)
	} else {
		mainLog.Info("startup health check succeeded")
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		mainLog.Error("failed to load config", "path", *configPath, "error", err)
		os.Exit(1)
	}
	mainLog.Info("config loaded", "services", len(cfg.Services), "path", *configPath)

	// Load the NixOS-managed registry and filter out any stale entries from
	// services.yaml that are no longer declared in the NixOS configuration.
	registry, err := loadManagedUnits(*registryPath)
	if err != nil {
		mainLog.Error("failed to load managed-units registry", "path", *registryPath, "error", err)
		os.Exit(1)
	}
	if registry == nil {
		mainLog.Warn("managed-units registry not found, running without scope validation", "path", *registryPath)
	} else {
		removed := filterByRegistry(cfg, registry)
		mainLog.Info("registry loaded", "managed_units", len(registry), "filtered_out", removed, "path", *registryPath)
	}

	// State directory is configured by the NixOS module (StateDirectory in
	// systemd). State lives here so user-stopped flags and the kernel boot
	// id survive a `nh os switch`. /run is tmpfs and would lose them.
	stateDir := *stateDirFlag
	if err := os.MkdirAll(stateDir, 0o750); err != nil {
		mainLog.Error("cannot create state dir", "error", err)
		os.Exit(1)
	}
	state := newState(stateDir+"/state.json", cfg)

	// Circuit breaker: tracks consecutive failures and enforces backoff.
	breaker := newCircuitBreaker()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	// ── Notifier (SMTP alerts) ───────────────────────────────────────
	notify := notifier.New(
		notifier.SMTPConfig{
			Host:     cfg.Notify.SMTP.Host,
			Port:     cfg.Notify.SMTP.Port,
			Username: cfg.Notify.SMTP.Username,
			Password: cfg.Notify.SMTP.Password,
		},
		cfg.Notify.From,
		cfg.Notify.To,
		hostname,
	)
	if notify.Enabled() {
		mainLog.Info("notifier configured", "host", cfg.Notify.SMTP.Host, "to", cfg.Notify.To)
	} else {
		mainLog.Info("notifier not configured (no SMTP host)")
	}

	// ── Daemon crash detection ──────────────────────────────────────
	// A marker file is written on clean shutdown and removed on crash.
	// If it exists at startup, the previous instance did not shut down cleanly.
	crashMarker := stateDir + "/.clean-shutdown"
	if _, err := os.Stat(crashMarker); err == nil {
		// Marker exists — previous shutdown was clean, remove it.
		os.Remove(crashMarker)
	} else if os.IsNotExist(err) {
		// No marker — possible crash. Only notify if state file exists
		// (meaning the daemon has run at least once before).
		if _, err := os.Stat(stateDir + "/state.json"); err == nil && notify.Enabled() {
			subj, body := notify.DaemonCrash()
			if err := notify.Send(subj, body); err != nil {
				mainLog.Warn("failed to send crash notification", "error", err)
			}
		}
	}

	// Write the clean-shutdown marker now; it gets removed in the defer
	// below. If we crash before the defer runs, the marker stays gone and
	// the next startup detects the crash.
	os.WriteFile(crashMarker, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o600)

	// Scheduler for cron backups
	scheduler := NewScheduler(ctx, cfg, state, notify)

	// Container updates checker background worker
	updatesMod := updates.New(stateDir)

	// Host statistics collector (CPU, memory, disk, network, GPU, processes).
	col := collector.NewCollector(
		collector.WithMounts([]string{"/"}),
	)

	// VPN subsystem (WireGuard netns + NAT-PMP). Maps parsed config → module.
	vpnMod := vpn.New(vpn.Config{
		Enabled:                cfg.VPN.Enabled,
		NetnsName:              cfg.VPN.NetnsName,
		Interface:              cfg.VPN.Interface,
		Address:                cfg.VPN.Address,
		DNS:                    cfg.VPN.DNS,
		PeerPublicKey:          cfg.VPN.PeerPublicKey,
		PeerEndpoint:           cfg.VPN.PeerEndpoint,
		AllowedIPs:             cfg.VPN.AllowedIPs,
		PrivateKeyFile:         cfg.VPN.PrivateKeyFile,
		Provider:               cfg.VPN.Provider,
		Type:                   cfg.VPN.Type,
		ServerCountry:          cfg.VPN.ServerCountry,
		VethHostIP:             cfg.VPN.VethHostIP,
		VethNetnsIP:            cfg.VPN.VethNetnsIP,
		PortFile:               cfg.VPN.PortFile,
		RefreshIntervalSeconds: cfg.VPN.RefreshIntervalSeconds,
	})

	// All long-running background workers share an errgroup so a fatal
	// failure in one (or context cancellation from SIGTERM) tears the
	// daemon down cleanly with proper goroutine shutdown.
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		updatesMod.Start(gctx)
		return nil
	})

	g.Go(func() error {
		col.Start(gctx, 2*time.Second)
		return nil
	})

	g.Go(func() error {
		vpnMod.Start(gctx)
		return nil
	})

	// Prometheus /metrics endpoint, loopback-only. Address overridable
	// via HOMELAB_METRICS_ADDR.
	metricsAddr := os.Getenv("HOMELAB_METRICS_ADDR")
	if metricsAddr == "" {
		metricsAddr = "127.0.0.1:9101"
	}
	g.Go(func() error {
		startMetricsServer(gctx, metricsAddr)
		return nil
	})

	// Start API server first so it's available during boot sequence. The
	// socket stays in /run/homelab-daemon — that path is hardcoded by every
	// consumer (homelab-dash, the `homelab` CLI, the module docs).
	sockPath := "/run/homelab-daemon/daemon.sock"
	g.Go(func() error {
		if err := serveAPI(gctx, sockPath, cfg, state, *configPath, breaker, scheduler, updatesMod, col, notify, vpnMod); err != nil {
			mainLog.Error("API server stopped", "error", err)
			return err
		}
		return nil
	})

	// Boot sequence: auto-mount storage, then start enabled services in order.
	g.Go(func() error {
		// Auto-mount pools first.
		if len(cfg.Storage.Pools) > 0 {
			mainLog.Info("discovering storage pools for auto-mount")
			pools, err := bcachefs.DiscoverPools()
			if err != nil {
				mainLog.Error("failed to discover bcachefs pools during boot", "error", err)
			} else {
				for _, pConf := range cfg.Storage.Pools {
					if !pConf.AutoMount {
						continue
					}
					var targetPool *bcachefs.Pool
					for i, p := range pools {
						if p.UUID == pConf.UUID {
							targetPool = &pools[i]
							break
						}
					}
					if targetPool == nil {
						mainLog.Error("auto-mount pool not found", "uuid", pConf.UUID)
						continue
					}
					if targetPool.State == "mounted" && targetPool.Mountdir == pConf.Mountpoint {
						mainLog.Info("pool already mounted", "uuid", pConf.UUID, "mountpoint", pConf.Mountpoint)
						continue
					}

					var devices []string
					for _, d := range targetPool.Disks {
						devices = append(devices, d.Path)
					}
					mainLog.Info("auto-mounting pool", "uuid", pConf.UUID, "mountpoint", pConf.Mountpoint)
					// Remove immutable flag — /pool is set +i at boot
					// to prevent writes to the bare directory when the
					// bcachefs pool is not mounted.
					cmdrunner.New("boot", "chattr", "-i", pConf.Mountpoint).Run()
					if err := bcachefs.Mount(devices, pConf.Mountpoint); err != nil {
						mainLog.Error("failed to auto-mount pool", "uuid", pConf.UUID, "error", err)
					}
				}
			}
		}

		// Always verify running services against their current constraints
		// on daemon startup — a config reload or mount change may have
		// invalidated previously-running services.
		if n := verifyRunningServices(cfg); n > 0 {
			mainLog.Info("constraint verification stopped running services", "count", n)
		}

		// Only run the ordered boot sequence if this is a brand-new kernel
		// session. A daemon-only restart (`nh os switch`, manual restart,
		// crash) reuses the same boot id and should leave running services
		// alone — they'll continue to be supervised by the monitor loop.
		if state.IsFirstStartAfterBoot() {
			boot(gctx, cfg, state)
		}
		return nil
	})

	// Monitor loop: restart services per policy with circuit breaker backoff.
	g.Go(func() error {
		monitor(gctx, cfg, state, breaker, notify)
		return nil
	})

	if err := g.Wait(); err != nil {
		mainLog.Error("daemon worker exited with error", "error", err)
	}
	mainLog.Info("homelab-daemon shutting down")
}
