package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	logging "github.com/ogglord/homelab-logging"
	"github.com/ogglord/homelab-daemon/internal/notifier"
	"github.com/robfig/cron/v3"
)

// hcClient is the shared HTTP client for Healthchecks.io pings. A 10s
// timeout keeps a slow/unreachable hc-ping.com from stalling a backup run.
var hcClient = &http.Client{Timeout: 10 * time.Second}
var schedulerLog = logging.Logger("api")

func pingHealthcheck(url string) {
	resp, err := hcClient.Get(url)
	if err != nil {
		schedulerLog.Warn("healthcheck ping failed", "url", url, "error", err)
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
}

type Scheduler struct {
	mu      sync.Mutex
	cron    *cron.Cron
	ctx     context.Context
	cfg     *Config
	state   *State
	notify  *notifier.Notifier
	entries map[string]cron.EntryID
	lastRun map[string]time.Time // unit → last run start (survives cron Prev resets on restart)
}

func NewScheduler(ctx context.Context, cfg *Config, state *State, notify *notifier.Notifier) *Scheduler {
	s := &Scheduler{
		ctx:     ctx,
		cfg:     cfg,
		state:   state,
		notify:  notify,
		entries: make(map[string]cron.EntryID),
		lastRun: make(map[string]time.Time),
	}
	s.Reload(cfg)
	return s
}

func (s *Scheduler) Reload(cfg *Config) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cron != nil {
		s.cron.Stop()
	}

	s.cfg = cfg
	s.cron = cron.New()
	s.entries = make(map[string]cron.EntryID)

	for _, b := range cfg.Backups {
		if !b.Enabled || b.Schedule == "" {
			continue
		}

		// Capture loop variable
		backup := b
		id, err := s.cron.AddFunc(backup.Schedule, func() {
			s.RunBackup(backup.Unit)
		})
		if err != nil {
			schedulerLog.Error("failed to schedule backup", "unit", backup.Unit, "schedule", backup.Schedule, "error", err)
		} else {
			s.entries[backup.Unit] = id
			schedulerLog.Info("scheduled backup", "unit", backup.Unit, "schedule", backup.Schedule)
		}
	}

	s.cron.Start()
}

func (s *Scheduler) GetEntry(unit string) *cron.Entry {
	s.mu.Lock()
	defer s.mu.Unlock()

	id, ok := s.entries[unit]
	if !ok {
		return nil
	}
	entry := s.cron.Entry(id)
	return &entry
}

// recordRun stores the last-run start time for a unit. Called from
// RunBackup so manual and cron-triggered runs both update the timestamp.
func (s *Scheduler) recordRun(unit string) {
	s.mu.Lock()
	s.lastRun[unit] = time.Now()
	s.mu.Unlock()
}

// LastRunStart returns the last recorded run start for a unit, or zero
// time if never recorded. This is the fallback when cron Prev (which
// resets on daemon restart) and systemctl ExecMainStartTimestamp (which
// systemd wipes on daemon-reload) are both unavailable.
func (s *Scheduler) LastRunStart(unit string) time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastRun[unit]
}

// RunBackup manually or via cron runs the backup workflow.
func (s *Scheduler) RunBackup(unit string) error {
	var backup *Backup
	s.mu.Lock()
	for _, b := range s.cfg.Backups {
		if b.Unit == unit {
			backup = &b
			break
		}
	}
	s.mu.Unlock()

	if backup == nil {
		return fmt.Errorf("backup job %q not found", unit)
	}

	// Record the start time so API consumers see it even when cron Prev
	// (which resets on daemon restart) and systemctl timestamps (which
	// systemd wipes on daemon-reload) are both unavailable.
	s.recordRun(unit)

	log := schedulerLog.With("unit", backup.Unit)

	for _, mnt := range backup.RequiresMounts {
		if !isMounted(mnt) {
			log.Warn("required mount not present, skipping backup", "mountpoint", mnt)
			return fmt.Errorf("mount %q not present", mnt)
		}
	}

	for _, dep := range backup.DependsOn {
		if !isActive(dep) {
			log.Warn("dependency not active, skipping backup", "dep", dep)
			return fmt.Errorf("dependency %q not active", dep)
		}
	}

	hcURL := ""
	if backup.HealthcheckUUID != "" {
		hcURL = fmt.Sprintf("https://hc-ping.com/%s", backup.HealthcheckUUID)
		pingHealthcheck(hcURL + "/start")
	}

	if backup.PauseService != "" {
		log.Info("pausing service", "service", backup.PauseService)
		stopUnit(backup.PauseService)
		defer func() {
			log.Info("resuming service", "service", backup.PauseService)
			startUnit(backup.PauseService)
		}()
	}

	log.Info("starting backup job")
	err := startUnit(backup.Unit)

	if err != nil {
		log.Error("backup job failed", "error", err)
		if hcURL != "" {
			pingHealthcheck(hcURL + "/fail")
		}
		if s.notify.Enabled() {
			subj, body := s.notify.BackupFailed(unit, err)
			if err2 := s.notify.Send(subj, body); err2 != nil {
				log.Warn("failed to send backup failure notification", "error", err2)
			}
		}
		return err
	}

	log.Info("backup job completed successfully")
	if hcURL != "" {
		pingHealthcheck(hcURL)
	}

	return nil
}
