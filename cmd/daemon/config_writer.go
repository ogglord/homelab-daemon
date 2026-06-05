package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// writeConfig serializes the config to YAML and writes it atomically to the path.
func writeConfig(path string, cfg *Config) error {
	var buf bytes.Buffer
	buf.WriteString("version: 1\n")

	if len(cfg.Storage.Pools) > 0 {
		buf.WriteString("storage:\n  pools:\n")
		for _, p := range cfg.Storage.Pools {
			buf.WriteString(fmt.Sprintf("    - uuid: %s\n", p.UUID))
			buf.WriteString(fmt.Sprintf("      mountpoint: %s\n", p.Mountpoint))
			if p.Options != "" {
				buf.WriteString(fmt.Sprintf("      options: %s\n", p.Options))
			}
			buf.WriteString(fmt.Sprintf("      auto_mount: %t\n", p.AutoMount))
		}
	}

	buf.WriteString("services:\n")
	for _, s := range cfg.Services {
		buf.WriteString(fmt.Sprintf("  - unit: %s\n", s.Unit))
		buf.WriteString(fmt.Sprintf("    enabled: %t\n", s.Enabled))
		buf.WriteString(fmt.Sprintf("    order: %d\n", s.Order))
		buf.WriteString(fmt.Sprintf("    boot_delay: %d\n", s.BootDelay))
		buf.WriteString(fmt.Sprintf("    restart_delay: %d\n", s.RestartDelay))
		if s.Restart != "" {
			buf.WriteString(fmt.Sprintf("    restart: %s\n", s.Restart))
		} else {
			buf.WriteString("    restart: no\n")
		}
		if len(s.DependsOn) > 0 {
			buf.WriteString("    depends_on:\n")
			for _, dep := range s.DependsOn {
				buf.WriteString(fmt.Sprintf("      - %s\n", dep))
			}
		}
		if len(s.RequiresMounts) > 0 {
			buf.WriteString("    requires_mount:\n")
			for _, rm := range s.RequiresMounts {
				buf.WriteString(fmt.Sprintf("      - %s\n", rm))
			}
		}
	}

	if len(cfg.Backups) > 0 {
		buf.WriteString("backups:\n")
		for _, b := range cfg.Backups {
			buf.WriteString(fmt.Sprintf("  - unit: %s\n", b.Unit))
			buf.WriteString(fmt.Sprintf("    enabled: %t\n", b.Enabled))
			if b.Schedule != "" {
				buf.WriteString(fmt.Sprintf("    schedule: %s\n", b.Schedule))
			}
			if b.HealthcheckUUID != "" {
				buf.WriteString(fmt.Sprintf("    healthcheck_uuid: %s\n", b.HealthcheckUUID))
			}
			if b.PauseService != "" {
				buf.WriteString(fmt.Sprintf("    pause_service: %s\n", b.PauseService))
			}
			if len(b.DependsOn) > 0 {
				buf.WriteString("    depends_on:\n")
				for _, dep := range b.DependsOn {
					buf.WriteString(fmt.Sprintf("      - %s\n", dep))
				}
			}
			if len(b.RequiresMounts) > 0 {
				buf.WriteString("    requires_mount:\n")
				for _, rm := range b.RequiresMounts {
					buf.WriteString(fmt.Sprintf("      - %s\n", rm))
				}
			}
		}
	}

	// Write atomically.
	tmpFile, err := os.CreateTemp(filepath.Dir(path), ".services.yaml.*")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()
	defer func() {
		if tmpFile != nil {
			tmpFile.Close()
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmpFile.Write(buf.Bytes()); err != nil {
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		tmpFile = nil
		return err
	}
	// Make sure group or others can read it (0644).
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return err
	}

	oldName := tmpName
	tmpFile = nil // prevent defer cleanup from deleting the renamed file
	return os.Rename(oldName, path)
}
