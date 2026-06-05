package api

// Backup is the static configuration for one backup job. Mirrors the
// daemon's services.yaml `backups:` section.
type Backup struct {
	Unit            string   `json:"unit" yaml:"unit"`
	Enabled         bool     `json:"enabled" yaml:"enabled"`
	Schedule        string   `json:"schedule" yaml:"schedule"` // cron expression
	DependsOn       []string `json:"depends_on" yaml:"depends_on"`
	RequiresMounts  []string `json:"requires_mount" yaml:"requires_mount"`
	HealthcheckUUID string   `json:"healthcheck_uuid" yaml:"healthcheck_uuid"`
	PauseService    string   `json:"pause_service" yaml:"pause_service"`
}

// BackupStatus is the runtime status emitted by GET /api/v1/backups.
// Fields are flat (not embedded from Backup) so the generated TypeScript
// interface has direct property access: backups[0].unit, not backups[0].Backup.unit.
type BackupStatus struct {
	Unit            string   `json:"unit"`
	Enabled         bool     `json:"enabled"`
	Schedule        string   `json:"schedule"` // cron expression
	DependsOn       []string `json:"depends_on"`
	RequiresMounts  []string `json:"requires_mount"`
	HealthcheckUUID string   `json:"healthcheck_uuid"`
	PauseService    string   `json:"pause_service"`
	ActiveState     string   `json:"active_state"`
	Result          string   `json:"result"`
	LastRunStart    string   `json:"last_run_start"`
	LastRunEnd      string   `json:"last_run_end"`
	NextRun         string   `json:"next_run"`
}

// PatchBackupRequest is the body for PATCH /api/v1/backups/{unit}.
type PatchBackupRequest struct {
	Enabled         *bool     `json:"enabled,omitempty"`
	Schedule        *string   `json:"schedule,omitempty"`
	HealthcheckUUID *string   `json:"healthcheck_uuid,omitempty"`
	PauseService    *string   `json:"pause_service,omitempty"`
	DependsOn       *[]string `json:"depends_on,omitempty"`
}

// PatchBackupResponse is the result of PATCH /api/v1/backups/{unit}.
type PatchBackupResponse struct {
	Success bool   `json:"success"`
	Backup  Backup `json:"backup"`
}
