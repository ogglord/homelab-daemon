package main

import (
	"context"
	"fmt"
	logging "github.com/ogglord/homelab-logging"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/ogglord/homelab-daemon/internal/notifier"
	"time"

	"github.com/ogglord/homelab-daemon/internal/cmdrunner"
)

// backoffSchedule defines extra wait time after N consecutive failures.
// Index 0 = after 1st failure, index 1 = after 2nd, etc. Capped at last value.
var monitorLog = logging.Logger("monitor")

var backoffSchedule = []time.Duration{
	0,                // 1st failure: no extra wait beyond RestartDelay
	30 * time.Second, // 2nd failure
	2 * time.Minute,  // 3rd failure
	10 * time.Minute, // 4th failure
	1 * time.Hour,    // 5th+ failure
}

// unitBreaker holds per-unit circuit-breaker state.
type unitBreaker struct {
	consecutive  int       // consecutive failures since last successful start
	backoffUntil time.Time // don't attempt restart before this time
}

// CircuitBreaker tracks restart failures per unit and enforces exponential backoff.
type CircuitBreaker struct {
	mu    sync.Mutex
	units map[string]*unitBreaker
}

func newCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{units: make(map[string]*unitBreaker)}
}

func (cb *CircuitBreaker) get(unit string) *unitBreaker {
	b, ok := cb.units[unit]
	if !ok {
		b = &unitBreaker{}
		cb.units[unit] = b
	}
	return b
}

// IsOpen returns true if the unit is still within its backoff window.
func (cb *CircuitBreaker) IsOpen(unit string) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return time.Now().Before(cb.get(unit).backoffUntil)
}

// State returns the current failure count and backoff-until time for a unit.
func (cb *CircuitBreaker) State(unit string) (consecutive int, backoffUntil time.Time) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	b := cb.get(unit)
	return b.consecutive, b.backoffUntil
}

// RecordFailure increments the failure counter and sets the next backoff window.
// Returns the backoff duration that was applied.
func (cb *CircuitBreaker) RecordFailure(unit string) time.Duration {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	b := cb.get(unit)
	b.consecutive++
	idx := b.consecutive - 1
	if idx >= len(backoffSchedule) {
		idx = len(backoffSchedule) - 1
	}
	d := backoffSchedule[idx]
	if d > 0 {
		b.backoffUntil = time.Now().Add(d)
		monitorLog.Warn("circuit breaker: backing off",
			"unit", unit,
			"consecutive_failures", b.consecutive,
			"backoff", d,
			"retry_at", b.backoffUntil.Format(time.TimeOnly),
		)
	}
	return d
}

// Reset clears the circuit breaker for a unit after a successful start.
func (cb *CircuitBreaker) Reset(unit string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if b, ok := cb.units[unit]; ok && b.consecutive > 0 {
		monitorLog.Info("circuit breaker: reset", "unit", unit, "was_consecutive", b.consecutive)
	}
	delete(cb.units, unit)
}

// unitOpMu serialises concurrent start/stop calls on the same unit.
// Keyed by unit name; values are *sync.Mutex. This prevents a monitor
// restart from racing with an API stop (or vice-versa) on the same unit.
var unitOpMu sync.Map

// lockUnit acquires the per-unit mutex and returns an unlock function.
func lockUnit(unit string) func() {
	v, _ := unitOpMu.LoadOrStore(unit, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// systemctlTimeout is the per-call timeout applied to every systemctl
// invocation so a hung systemd transaction (e.g. waiting on a dead D-Bus
// peer) cannot stall the monitor loop or an API request indefinitely.
const systemctlTimeout = 30 * time.Second

// systemctlCtx returns a context with the per-call timeout. It derives from
// context.Background() rather than threading ctx through every caller —
// that broader refactor lives with the package-main split.
func systemctlCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), systemctlTimeout)
}

// isActive reports whether a systemd unit is currently active.
func isActive(unit string) bool {
	ctx, cancel := systemctlCtx()
	defer cancel()
	_, err := cmdrunner.New("monitor", "systemctl", "is-active", "--quiet", unit).
		WithContext(ctx).
		Run()
	return err == nil
}

// isActiveMultiple reports the active status of multiple systemd units in a single command.
func isActiveMultiple(units []string) map[string]bool {
	res := make(map[string]bool, len(units))
	if len(units) == 0 {
		return res
	}

	ctx, cancel := systemctlCtx()
	defer cancel()
	args := append([]string{"is-active"}, units...)
	cmdRes, _ := cmdrunner.New("monitor", "systemctl", args...).
		WithContext(ctx).
		Run()

	lines := strings.Split(strings.TrimSpace(cmdRes.Stdout), "\n")
	if len(lines) != len(units) {
		monitorLog.Warn("systemctl is-active multiple output length mismatch, falling back to individual checks", "expected", len(units), "got", len(lines))
		for _, unit := range units {
			res[unit] = isActive(unit)
		}
		return res
	}

	for i, unit := range units {
		res[unit] = strings.TrimSpace(lines[i]) == "active"
	}
	return res
}

// checkConstraints returns an error if the service cannot be started due
// to missing mounts. Returns nil if all constraints are satisfied.
// This is the single validation point used by boot, monitor, and the API.
func checkConstraints(svc Service) error {
	for _, mnt := range svc.RequiresMounts {
		if !isMounted(mnt) {
			return fmt.Errorf("mount %q not mounted", mnt)
		}
	}
	return nil
}

// verifyRunningServices checks all currently-running managed services
// against their declared constraints (mounts). If a running service has
// unmet constraints — because the config was reloaded or a mount was
// unmounted — it is stopped. Returns the count of services stopped.
func verifyRunningServices(cfg *Config) int {
	stopped := 0
	for _, svc := range cfg.Services {
		if !svc.Enabled || !isActive(svc.Unit) {
			continue
		}
		if err := checkConstraints(svc); err != nil {
			monitorLog.Warn("stopping running service with unmet constraints",
				"unit", svc.Unit, "error", err)
			if stopErr := stopUnit(svc.Unit); stopErr != nil {
				monitorLog.Error("failed to stop service with unmet constraints",
					"unit", svc.Unit, "stop_error", stopErr)
			} else {
				stopped++
			}
		}
	}
	return stopped
}

// isMounted reports whether the given path is present as a mount target in /proc/mounts.
func isMounted(path string) bool {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}
	// /proc/mounts format: device mountpoint fstype options freq passno
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			if fields[1] == path {
				return true
			}
		}
	}
	return false
}

// startUnit calls "systemctl start <unit>".
// Acquires the per-unit mutex so concurrent monitor restarts and API calls
// cannot issue overlapping systemctl start/stop on the same unit.
func startUnit(unit string) error {
	unlock := lockUnit(unit)
	defer unlock()
	ctx, cancel := systemctlCtx()
	defer cancel()
	res, err := cmdrunner.New("monitor", "systemctl", "start", unit).
		WithContext(ctx).
		Output(cmdrunner.OutputCombined).
		Run()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(res.Output))
	}
	return nil
}

// stopUnit calls "systemctl stop <unit>" and, if the unit ends up in the
// systemd "failed" state (common when an upstream unit lacks
// SuccessExitStatus= for the signal/exit it actually exits with — Plex's
// SIGQUIT-then-coredump, podman containers exiting 143 on SIGTERM, etc.),
// clears that failed state so the post-stop view reads "inactive". A
// user-initiated stop is always a clean stop from our perspective.
func stopUnit(unit string) error {
	unlock := lockUnit(unit)
	defer unlock()
	ctx, cancel := systemctlCtx()
	defer cancel()
	res, err := cmdrunner.New("monitor", "systemctl", "stop", unit).
		WithContext(ctx).
		Output(cmdrunner.OutputCombined).
		Run()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(res.Output))
	}
	if activeState(unit) == "failed" {
		rfCtx, rfCancel := systemctlCtx()
		defer rfCancel()
		if _, rfErr := cmdrunner.New("monitor", "systemctl", "reset-failed", unit).
			WithContext(rfCtx).
			Run(); rfErr != nil {
			monitorLog.Warn("reset-failed after stop failed", "unit", unit, "error", rfErr)
		} else {
			monitorLog.Info("cleared spurious failed state after stop", "unit", unit)
		}
	}
	return nil
}

// activeState returns the ActiveState property of a systemd unit (active,
// inactive, failed, activating, deactivating, …). Empty string on error.
func activeState(unit string) string {
	ctx, cancel := systemctlCtx()
	defer cancel()
	res, err := cmdrunner.New("monitor", "systemctl", "show", unit, "--property=ActiveState", "--value").
		WithContext(ctx).
		Run()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(res.Stdout)
}

// restartUnit calls "systemctl restart <unit>".
func restartUnit(unit string) error {
	ctx, cancel := systemctlCtx()
	defer cancel()
	res, err := cmdrunner.New("monitor", "systemctl", "restart", unit).
		WithContext(ctx).
		Output(cmdrunner.OutputCombined).
		Run()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(res.Output))
	}
	return nil
}

// waitActive polls until the unit is active or the deadline is exceeded.
func waitActive(ctx context.Context, unit string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if isActive(unit) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for %s to become active", unit)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

// boot runs the ordered startup sequence for all enabled services.
func boot(ctx context.Context, cfg *Config, state *State) {
	enabled := make([]Service, 0, len(cfg.Services))
	for _, svc := range cfg.Services {
		if !svc.Enabled {
			continue
		}
		// A user-initiated stop is sticky across daemon restarts regardless
		// of policy. Otherwise pressing "Stop All" and then doing anything
		// that restarts the daemon (a rebuild, a manual restart) would
		// silently bring `on-failure` / `always` services back up.
		if state.IsUserStopped(svc.Unit) {
			monitorLog.Info("skipping boot start (user-stopped)", "unit", svc.Unit, "policy", svc.Restart)
			continue
		}
		enabled = append(enabled, svc)
	}

	sort.Slice(enabled, func(i, j int) bool {
		return enabled[i].Order < enabled[j].Order
	})

	for _, svc := range enabled {
		if ctx.Err() != nil {
			return
		}
		log := monitorLog.With("unit", svc.Unit, "order", svc.Order)

		// Wait for all depends_on units to become active.
		for _, dep := range svc.DependsOn {
			log.Info("waiting for dependency", "dep", dep)
			if err := waitActive(ctx, dep, 2*time.Minute); err != nil {
				log.Error("dependency not ready, skipping service", "dep", dep, "error", err)
				goto next
			}
		}

		// Fix C: re-verify all deps are still active immediately before starting.
		// A dependency can crash between waitActive returning and startUnit being
		// called, especially when boot_delay is non-zero.
		for _, dep := range svc.DependsOn {
			if !isActive(dep) {
				log.Error("dependency became inactive before start, skipping service", "dep", dep)
				goto next
			}
		}

		// Check constraints (mounts).
		if err := checkConstraints(svc); err != nil {
			log.Error("constraint check failed, skipping service", "error", err)
			goto next
		}

		// Optional boot delay.
		if svc.BootDelay > 0 {
			log.Info("boot delay", "seconds", svc.BootDelay)
			select {
			case <-time.After(time.Duration(svc.BootDelay) * time.Second):
			case <-ctx.Done():
				return
			}
		}

		log.Info("starting")
		if err := startUnit(svc.Unit); err != nil {
			log.Error("start failed", "error", err)
		} else {
			log.Info("started ok")
		}

	next:
	}
}

// monitor polls every 5 seconds and restarts services per their restart policy.
// It uses a CircuitBreaker to apply exponential backoff after consecutive failures.
func monitor(ctx context.Context, cfg *Config, state *State, breaker *CircuitBreaker, notify *notifier.Notifier) {
	prevFailures := make(map[string]int)
	// inFlight tracks units that already have a restart goroutine running,
	// preventing duplicate restarts when a unit takes longer than 5s to settle.
	var inFlight sync.Map

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// ── Service failure notification ──────────────────────────
			for _, svc := range cfg.Services {
				if !svc.Enabled {
					continue
				}
				consecutive, _ := breaker.State(svc.Unit)
				if consecutive > prevFailures[svc.Unit] && consecutive > 0 && notify.Enabled() {
					name := strings.TrimPrefix(svc.Unit, "podman-")
					name = strings.TrimSuffix(name, ".service")
					subj, body := notify.ServiceFailed(svc.Unit, name, fmt.Sprintf("%d consecutive failures", consecutive))
					if err := notify.SendWithCooldown("service-failure", time.Hour, subj, body); err != nil {
						monitorLog.Warn("failed to send service failure notification", "unit", svc.Unit, "error", err)
					}
				}
				prevFailures[svc.Unit] = consecutive
			}

			var unitsToCheck []string
			for _, svc := range cfg.Services {
				if !svc.Enabled || svc.Restart == "" || svc.Restart == "no" {
					continue
				}
				unitsToCheck = append(unitsToCheck, svc.Unit)
			}

			activeStates := isActiveMultiple(unitsToCheck)

			for _, svc := range cfg.Services {
				if !svc.Enabled || svc.Restart == "" || svc.Restart == "no" {
					continue
				}
				if activeStates[svc.Unit] {
					// Service is healthy — reset any circuit breaker state.
					breaker.Reset(svc.Unit)
					continue
				}
				if !shouldRestart(svc, state) {
					continue
				}
				if breaker.IsOpen(svc.Unit) {
					consecutive, until := breaker.State(svc.Unit)
					monitorLog.Debug("circuit breaker open, skipping restart",
						"unit", svc.Unit,
						"consecutive", consecutive,
						"retry_at", until.Format(time.TimeOnly),
					)
					continue
				}

				// Check constraints before restarting.
				if err := checkConstraints(svc); err != nil {
					monitorLog.Debug("constraint check failed, skipping restart", "unit", svc.Unit, "error", err)
					continue
				}

				// Skip if a restart goroutine is already running for this unit.
				if _, loaded := inFlight.LoadOrStore(svc.Unit, struct{}{}); loaded {
					monitorLog.Debug("restart already in flight, skipping", "unit", svc.Unit)
					continue
				}

				go func(s Service) {
					defer inFlight.Delete(s.Unit)
					restart(ctx, s, breaker)
				}(svc)
			}
		}
	}
}

// shouldRestart returns true if the policy and current state warrant a restart.
func shouldRestart(svc Service, state *State) bool {
	switch svc.Restart {
	case "always":
		return true
	case "unless-stopped":
		return !state.IsUserStopped(svc.Unit)
	case "on-failure":
		if state.IsUserStopped(svc.Unit) {
			return false
		}
		return exitedWithFailure(svc.Unit)
	}
	return false
}

// exitedWithFailure reports whether the unit's last run ended with a non-zero exit.
func exitedWithFailure(unit string) bool {
	ctx, cancel := systemctlCtx()
	defer cancel()
	res, err := cmdrunner.New("monitor", "systemctl", "show", unit, "--property=ExecMainStatus").
		WithContext(ctx).
		Run()
	if err != nil {
		return false
	}
	line := strings.TrimSpace(res.Stdout)
	return line != "ExecMainStatus=0" && line != "ExecMainStatus="
}

// restart waits the configured delay then attempts to start the unit,
// recording success or failure in the circuit breaker.
func restart(ctx context.Context, svc Service, breaker *CircuitBreaker) {
	log := monitorLog.With("unit", svc.Unit, "policy", svc.Restart)
	log.Info("service inactive, scheduling restart", "delay_s", svc.RestartDelay)

	if svc.RestartDelay > 0 {
		select {
		case <-time.After(time.Duration(svc.RestartDelay) * time.Second):
		case <-ctx.Done():
			return
		}
	}

	// Fix A: verify depends_on before restarting. A missing dependency is a
	// constraint miss, not a service failure — don't record it in the breaker.
	for _, dep := range svc.DependsOn {
		if !isActive(dep) {
			log.Info("restart skipped — dependency inactive", "dep", dep)
			return
		}
	}

	if err := startUnit(svc.Unit); err != nil {
		log.Error("restart failed", "error", err)
		backoff := breaker.RecordFailure(svc.Unit)
		if backoff > 0 {
			log.Warn("will back off before next attempt", "backoff", backoff)
		}
		return
	}

	// systemctl start returns 0 once the unit leaves "activating", but
	// failure-prone units may die seconds later (bad config, missing dep,
	// container that exits on its own entrypoint error). Wait for the unit
	// to settle, then verify it is still active before clearing the
	// breaker — otherwise the counter resets every cycle and backoff never
	// escalates.
	const settle = 10 * time.Second
	select {
	case <-time.After(settle):
	case <-ctx.Done():
		return
	}
	if !isActive(svc.Unit) {
		log.Warn("started but did not stay active", "settle", settle)
		backoff := breaker.RecordFailure(svc.Unit)
		if backoff > 0 {
			log.Warn("will back off before next attempt", "backoff", backoff)
		}
		return
	}

	log.Info("restarted ok")
	breaker.Reset(svc.Unit)
}
