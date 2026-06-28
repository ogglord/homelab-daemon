// Package main — homelab-daemon configuration parser.
package main

import (
	"fmt"
	"os"
	"strings"

	logging "github.com/ogglord/homelab-logging"
	"gopkg.in/yaml.v3"
)

var configLog = logging.Logger("api")

const CurrentSchemaVersion = 1

// Config is the top-level structure of services.yaml.
type Config struct {
	Version  int           `yaml:"version"`
	Notify   NotifyConfig  `yaml:"notify"`
	Storage  StorageConfig `yaml:"storage"`
	VPN      VPNConfig     `yaml:"vpn"`
	Services []Service     `yaml:"services"`
	Backups  []Backup      `yaml:"backups"`
}

// VPNConfig configures the daemon-owned WireGuard netns. All values are
// generic VPN infrastructure — the daemon never references any consumer.
type VPNConfig struct {
	Enabled                bool   `yaml:"enabled"`
	NetnsName              string `yaml:"netns_name"`
	Interface              string `yaml:"interface"`
	Address                string `yaml:"address"`
	DNS                    string `yaml:"dns"`
	PeerPublicKey          string `yaml:"peer_public_key"`
	PeerEndpoint           string `yaml:"peer_endpoint"`
	AllowedIPs             string `yaml:"allowed_ips"`
	PrivateKeyFile         string `yaml:"private_key_file"`
	Provider               string `yaml:"provider"`
	Type                   string `yaml:"type"`
	ServerCountry          string `yaml:"server_country"`
	VethHostIP             string `yaml:"veth_host_ip"`
	VethNetnsIP            string `yaml:"veth_netns_ip"`
	PortFile               string `yaml:"port_file"`
	RefreshIntervalSeconds int    `yaml:"refresh_interval_seconds"`
}

// NotifyConfig holds SMTP alert configuration.
type NotifyConfig struct {
	SMTP SMTPConfig `yaml:"smtp"`
	From string     `yaml:"from"`
	To   string     `yaml:"to"`
}

// SMTPConfig for sending email alerts.
type SMTPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// StorageConfig holds user-managed storage settings.
type StorageConfig struct {
	Pools []StoragePoolConfig `yaml:"pools"`
}

// StoragePoolConfig defines auto-mount and parameters for a bcachefs pool.
type StoragePoolConfig struct {
	UUID       string `yaml:"uuid" json:"uuid"`
	Mountpoint string `yaml:"mountpoint" json:"mountpoint"`
	Options    string `yaml:"options" json:"options"`
	AutoMount  bool   `yaml:"auto_mount" json:"auto_mount"`
	Name       string `yaml:"name,omitempty" json:"name,omitempty"`
}

// Service describes one managed systemd unit.
type Service struct {
	Unit           string   `yaml:"unit"`           // systemd unit name, e.g. "immich-server.service"
	Enabled        bool     `yaml:"enabled"`        // whether the daemon manages this service
	Order          int      `yaml:"order"`          // boot order — lower starts first
	BootDelay      int      `yaml:"boot_delay"`     // seconds to wait after depends_on before starting
	DependsOn      []string `yaml:"depends_on"`     // other units that must be active first
	RequiresMounts []string `yaml:"requires_mount"` // mountpoints that must be mounted first
	Restart        string   `yaml:"restart"`        // no | on-failure | unless-stopped | always
	RestartDelay   int      `yaml:"restart_delay"`  // seconds to wait before restarting
	IconURL        string   `yaml:"icon_url"`       // service icon URL (selfh.st or custom)
	HomepageURL    string   `yaml:"homepage_url"`   // frontend link, e.g. https://immich.cignl.cc
}

// Backup describes a daemon-managed backup job.
type Backup struct {
	Unit            string   `yaml:"unit"` // systemd unit name, e.g. "b2-backup-appdata.service"
	Enabled         bool     `yaml:"enabled"`
	Schedule        string   `yaml:"schedule"` // cron expression
	DependsOn       []string `yaml:"depends_on"`
	RequiresMounts  []string `yaml:"requires_mount"`
	HealthcheckUUID string   `yaml:"healthcheck_uuid"` // healthchecks.io UUID
	PauseService    string   `yaml:"pause_service"`    // service to stop before, start after
}

// loadConfig reads and parses the YAML config file.
func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	cfg, err := parseConfig(data)
	if err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	for i, s := range cfg.Services {
		if s.Unit == "" {
			return nil, fmt.Errorf("service at index %d has no unit", i)
		}
		switch s.Restart {
		case "", "no", "on-failure", "unless-stopped", "always":
		default:
			return nil, fmt.Errorf("service %q: invalid restart %q", s.Unit, s.Restart)
		}
	}
	return cfg, nil
}

// parseConfig decodes yaml using the gopkg.in/yaml.v3 library.
func parseConfig(data []byte) (*Config, error) {
	var cfg Config
	// Defaults
	cfg.Version = 0
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Migrate upgrades old config structures to CurrentSchemaVersion.
// Returns true if any changes (migrations) were applied.
func Migrate(cfg *Config) bool {
	modified := false
	if cfg.Version < 1 {
		cfg.Version = 1
		modified = true
	}
	// Future schema migrations will go here, e.g.:
	// if cfg.Version < 2 {
	//     migrateToV2(cfg)
	//     modified = true
	// }
	return modified
}

// loadManagedUnits reads the NixOS-generated registry file at the given path
// and returns a set of unit names that are authorised to be daemon-managed.
func loadManagedUnits(path string) (map[string]struct{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read managed-units registry %q: %w", path, err)
	}
	units := make(map[string]struct{})
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			units[line] = struct{}{}
		}
	}
	return units, nil
}

// filterByRegistry removes services from cfg that are not present in the
// managed-units registry. If registry is nil, no filtering is applied.
// Returns the number of entries that were removed.
func filterByRegistry(cfg *Config, registry map[string]struct{}) int {
	if registry == nil {
		return 0
	}
	kept := cfg.Services[:0]
	removed := 0
	for _, svc := range cfg.Services {
		if _, ok := registry[svc.Unit]; ok {
			kept = append(kept, svc)
		} else {
			configLog.Warn("service not in managed-units registry, skipping",
				"unit", svc.Unit,
				"hint", "run nh os switch to update /etc/homelab-daemon/managed-units")
			removed++
		}
	}
	cfg.Services = kept
	return removed
}
