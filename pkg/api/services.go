package api

// Service is the static configuration for one managed systemd unit.
// Mirrors the daemon's services.yaml. Emitted by GET /api/v1/config.
type Service struct {
	Unit           string   `json:"unit" yaml:"unit"`
	Enabled        bool     `json:"enabled" yaml:"enabled"`
	Order          int      `json:"order" yaml:"order"`
	BootDelay      int      `json:"boot_delay" yaml:"boot_delay"`
	DependsOn      []string `json:"depends_on" yaml:"depends_on"`
	RequiresMounts []string `json:"requires_mount" yaml:"requires_mount"`
	Restart        string   `json:"restart" yaml:"restart"`
	RestartDelay   int      `json:"restart_delay" yaml:"restart_delay"`
	IconURL        string   `json:"icon_url,omitempty" yaml:"icon_url,omitempty"`
	HomepageURL    string   `json:"homepage_url,omitempty" yaml:"homepage_url,omitempty"`
}

// ServiceStatus is the per-unit runtime status emitted by
// GET /api/v1/status. It is the **superset** of fields the dash and
// frontend already expect; the daemon populates all of them.
type ServiceStatus struct {
	Unit           string   `json:"unit"`
	Enabled        bool     `json:"enabled"`
	Active         bool     `json:"active"`
	ActiveState    string   `json:"active_state"`
	SubState       string   `json:"sub_state"`
	Description    string   `json:"description"`
	UserStopped    bool     `json:"user_stopped"`
	Restart        string   `json:"restart"`
	Order          int      `json:"order"`
	BootDelay      int      `json:"boot_delay"`
	RestartDelay   int      `json:"restart_delay"`
	DependsOn      []string `json:"depends_on"`
	RequiresMounts []string `json:"requires_mount"`
	FailureCount   int      `json:"failure_count"`  // consecutive failures since last success
	BackoffUntil   string   `json:"backoff_until"`  // RFC3339; empty if not backing off
	BlockedReason  string   `json:"blocked_reason"` // why the daemon won't start/restart this service
	IconURL        string   `json:"icon_url,omitempty"`
	HomepageURL    string   `json:"homepage_url,omitempty"`
}

// PatchServiceRequest is the JSON body for PATCH /api/v1/config/{unit}.
// Pointer fields are tri-state: nil = leave unchanged, &"" = clear.
type PatchServiceRequest struct {
	Enabled      *bool     `json:"enabled,omitempty"`
	Order        *int      `json:"order,omitempty"`
	BootDelay    *int      `json:"boot_delay,omitempty"`
	DependsOn    *[]string `json:"depends_on,omitempty"`
	Restart      *string   `json:"restart,omitempty"`
	RestartDelay *int      `json:"restart_delay,omitempty"`
}

// PatchServiceResponse is the result of PATCH /api/v1/config/{unit}.
type PatchServiceResponse struct {
	Success bool    `json:"success"`
	Service Service `json:"service"`
}

// ServiceInfo is a superset of ServiceStatus that includes update metadata
// and config fields. It is the single source of truth for the frontend's
// service table, emitted by the daemon's merged services endpoint.
type ServiceInfo struct {
	Name            string   `json:"name"`
	Type            string   `json:"type"` // "Docker" or "Native"
	Image           string   `json:"image,omitempty"`
	ActiveState     string   `json:"active_state"`
	SubState        string   `json:"sub_state"`
	Description     string   `json:"description"`
	UnitName        string   `json:"unit_name"`
	UpdateAvailable bool     `json:"update_available"`
	CurrentVersion  string   `json:"current_version,omitempty"`
	RemoteVersion   string   `json:"remote_version,omitempty"`
	DaemonManaged   bool     `json:"daemon_managed"`
	RestartPolicy   string   `json:"restart_policy"`
	BootOrder       int      `json:"boot_order"`
	BootDelay       int      `json:"boot_delay"`
	RestartDelay    int      `json:"restart_delay"`
	DependsOn       []string `json:"depends_on"`
	UserStopped     bool     `json:"user_stopped"`
	DaemonEnabled   bool     `json:"daemon_enabled"`
	FailureCount    int      `json:"failure_count"`
	BackoffSeconds  int      `json:"backoff_seconds"`
	BlockedReason   string   `json:"blocked_reason"`  // why the daemon won't start/restart this service
	RequiresMounts  []string `json:"requires_mount"`  // mountpoints that must be present
	IconURL         string   `json:"icon_url,omitempty"`
	HomepageURL     string   `json:"homepage_url,omitempty"`
}
