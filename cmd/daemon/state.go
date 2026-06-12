package main

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"

	logging "github.com/ogglord/homelab-logging"
)

var stateLog = logging.Logger("api")

// State tracks which units were intentionally stopped through the daemon API.
// It is persisted to disk so "unless-stopped" survives daemon and host reboots.
type State struct {
	mu            sync.RWMutex
	userStopped   map[string]bool
	bootID        string            // last-seen kernel boot id; "" until loaded
	backupLastRun map[string]time.Time // unit → last backup run start time
	path          string
}

type stateFile struct {
	UserStopped   []string          `json:"user_stopped"`
	BootID        string            `json:"boot_id,omitempty"`
	BackupLastRun map[string]string `json:"backup_last_run,omitempty"` // unit → RFC3339
}

// newState loads existing state from disk (if any) and returns a State.
func newState(path string, cfg *Config) *State {
	s := &State{
		userStopped:   make(map[string]bool),
		backupLastRun: make(map[string]time.Time),
		path:          path,
	}
	s.load()
	// Prune stale entries: only keep units that are still in config.
	known := make(map[string]bool, len(cfg.Services))
	for _, svc := range cfg.Services {
		known[svc.Unit] = true
	}
	s.mu.Lock()
	for unit := range s.userStopped {
		if !known[unit] {
			delete(s.userStopped, unit)
		}
	}
	s.mu.Unlock()
	return s
}

// IsUserStopped returns true if the unit was explicitly stopped via the daemon.
func (s *State) IsUserStopped(unit string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.userStopped[unit]
}

// SetUserStopped marks or unmarks a unit as user-stopped and persists.
func (s *State) SetUserStopped(unit string, stopped bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if stopped {
		s.userStopped[unit] = true
	} else {
		delete(s.userStopped, unit)
	}
	s.persist()
}

// SetAllUserStopped bulk-marks all units and persists once.
func (s *State) SetAllUserStopped(units []string, stopped bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range units {
		if stopped {
			s.userStopped[u] = true
		} else {
			delete(s.userStopped, u)
		}
	}
	s.persist()
}

// SetBackupLastRun records the last run start time for a backup unit and persists.
func (s *State) SetBackupLastRun(unit string, t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backupLastRun[unit] = t
	s.persist()
}

// BackupLastRunTime returns the persisted last-run start time for a backup unit,
// or zero time if not recorded.
func (s *State) BackupLastRunTime(unit string) time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.backupLastRun[unit]
}

// BackupLastRunAll returns a copy of all persisted backup last-run times.
func (s *State) BackupLastRunAll() map[string]time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]time.Time, len(s.backupLastRun))
	for k, v := range s.backupLastRun {
		out[k] = v
	}
	return out
}

// UserStoppedList returns a snapshot of all user-stopped unit names.
func (s *State) UserStoppedList() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.userStopped))
	for u := range s.userStopped {
		out = append(out, u)
	}
	return out
}

// IsFirstStartAfterBoot returns true if the daemon process is starting in a
// kernel session it hasn't seen before — i.e. a real system boot, not a
// daemon-only restart (nh os switch, manual systemctl restart, crash). It
// also stamps the current boot id into state so subsequent calls within the
// same kernel session return false.
//
// The kernel hands out a fresh UUID at /proc/sys/kernel/random/boot_id each
// boot. By comparing to the value we persisted last time we ran, we can tell
// these two scenarios apart without any heuristics on uptime.
func (s *State) IsFirstStartAfterBoot() bool {
	current := readKernelBootID()
	if current == "" {
		// Can't read /proc — be conservative and treat as a real boot.
		stateLog.Warn("could not read kernel boot id, treating start as a system boot")
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prior := s.bootID
	first := prior != current
	s.bootID = current
	s.persist()
	if first {
		stateLog.Info("first start in new kernel session", "boot_id", current, "prior", prior)
	} else {
		stateLog.Info("daemon-only restart (same kernel session, boot() skipped)", "boot_id", current)
	}
	return first
}

func readKernelBootID() string {
	b, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// persist writes the current state to disk (caller must hold mu).
func (s *State) persist() {
	sf := stateFile{BootID: s.bootID}
	for u := range s.userStopped {
		sf.UserStopped = append(sf.UserStopped, u)
	}
	if len(s.backupLastRun) > 0 {
		sf.BackupLastRun = make(map[string]string, len(s.backupLastRun))
		for u, t := range s.backupLastRun {
			sf.BackupLastRun[u] = t.Format(time.RFC3339)
		}
	}
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		stateLog.Warn("state persist failed", "error", err)
	}
}

// load reads persisted state from disk (ignores missing file).
func (s *State) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return // fresh start
	}
	var sf stateFile
	if err := json.Unmarshal(data, &sf); err != nil {
		stateLog.Warn("state load failed, starting fresh", "error", err)
		return
	}
	for _, u := range sf.UserStopped {
		s.userStopped[u] = true
	}
	s.bootID = sf.BootID
	for u, ts := range sf.BackupLastRun {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			s.backupLastRun[u] = t
		}
	}
	stateLog.Info("state restored", "user_stopped", len(s.userStopped), "boot_id", s.bootID, "backup_last_run", len(s.backupLastRun))
}
