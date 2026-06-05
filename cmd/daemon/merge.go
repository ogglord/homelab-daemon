// Package main — homelab-daemon config merge subcommand.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	logging "github.com/ogglord/homelab-logging"
	"gopkg.in/yaml.v3"
)

var mergeLog = logging.Logger("api")

// handleMergeConfig merges a default configuration template into the existing user-controlled services.yaml.
func handleMergeConfig() {
	fs := flag.NewFlagSet("merge-config", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to the existing services.yaml config file")
	defaultPath := fs.String("defaults", "", "Path to the NixOS-generated defaults services.yaml")
	fs.Parse(os.Args[2:])

	if *configPath == "" || *defaultPath == "" {
		mergeLog.Error("merge-config: both --config and --defaults must be specified")
		fmt.Fprintln(os.Stderr, "Usage: homelab-daemon merge-config --config <path> --defaults <path>")
		os.Exit(1)
	}

	// 1. Load existing config.
	// If the file does not exist, copy defaults directly, apply migrations, and write.
	existingCfg, err := loadConfig(*configPath)
	if err != nil {
		if os.IsNotExist(err) {
			mergeLog.Info("services.yaml not found, bootstrapping from defaults")
			bootstrapFromDefaults(*defaultPath, *configPath)
			return
		}
		mergeLog.Error("reading existing config", "error", err)
		os.Exit(1)
	}

	// 2. Load NixOS default/computed config.
	defaultCfg, err := loadConfig(*defaultPath)
	if err != nil {
		mergeLog.Error("reading defaults config", "error", err)
		os.Exit(1)
	}

	// 3. Perform the merge.
	modified := false

	// Map existing services for quick lookup/index.
	existingServicesMap := make(map[string]int)
	for i, s := range existingCfg.Services {
		existingServicesMap[s.Unit] = i
	}

	// Append missing services from defaultCfg, and sync structural
	// fields (requires_mount, depends_on) for existing ones.
	for _, s := range defaultCfg.Services {
		if idx, ok := existingServicesMap[s.Unit]; ok {
			// Service exists — sync structural requirements from Nix.
			// These are authoritative in the Nix module; user edits
			// to order, restart, boot_delay etc. are preserved.
			existing := &existingCfg.Services[idx]
			if !stringSlicesEqual(existing.RequiresMounts, s.RequiresMounts) {
				existing.RequiresMounts = s.RequiresMounts
				mergeLog.Info("synced requires_mount for existing service", "unit", s.Unit, "requires_mount", s.RequiresMounts)
				modified = true
			}
			if !stringSlicesEqual(existing.DependsOn, s.DependsOn) {
				existing.DependsOn = s.DependsOn
				mergeLog.Info("synced depends_on for existing service", "unit", s.Unit, "depends_on", s.DependsOn)
				modified = true
			}
			if existing.IconURL != s.IconURL {
				existing.IconURL = s.IconURL
				mergeLog.Info("synced icon_url for existing service", "unit", s.Unit, "icon_url", s.IconURL)
				modified = true
			}
			if existing.HomepageURL != s.HomepageURL {
				existing.HomepageURL = s.HomepageURL
				mergeLog.Info("synced homepage_url for existing service", "unit", s.Unit, "homepage_url", s.HomepageURL)
				modified = true
			}
		} else {
			existingCfg.Services = append(existingCfg.Services, s)
			mergeLog.Info("merged new service into services.yaml", "unit", s.Unit)
			modified = true
		}
	}

	// Map of existing backups for quick lookup.
	existingBackupsMap := make(map[string]int)
	for i, b := range existingCfg.Backups {
		existingBackupsMap[b.Unit] = i
	}

	// Append missing backups from defaultCfg, sync structural fields for existing.
	for _, b := range defaultCfg.Backups {
		if idx, ok := existingBackupsMap[b.Unit]; ok {
			existing := &existingCfg.Backups[idx]
			if !stringSlicesEqual(existing.RequiresMounts, b.RequiresMounts) {
				existing.RequiresMounts = b.RequiresMounts
				mergeLog.Info("synced requires_mount for existing backup", "unit", b.Unit, "requires_mount", b.RequiresMounts)
				modified = true
			}
			if !stringSlicesEqual(existing.DependsOn, b.DependsOn) {
				existing.DependsOn = b.DependsOn
				mergeLog.Info("synced depends_on for existing backup", "unit", b.Unit, "depends_on", b.DependsOn)
				modified = true
			}
		} else {
			existingCfg.Backups = append(existingCfg.Backups, b)
			mergeLog.Info("merged new backup into services.yaml", "unit", b.Unit)
			modified = true
		}
	}

	// 4. Enforce migrations/versioning.
	if Migrate(existingCfg) {
		modified = true
	}

	// 5. Write back to disk if modified.
	if modified {
		if err := saveConfig(*configPath, existingCfg); err != nil {
			mergeLog.Error("saving merged configuration", "error", err)
			os.Exit(1)
		}
	} else {
		mergeLog.Info("services.yaml is up to date, no changes needed")
	}
}

// bootstrapFromDefaults initializes the config file by copying the defaults, applying migrations, and saving.
func bootstrapFromDefaults(defaultPath, configPath string) {
	cfg, err := loadConfig(defaultPath)
	if err != nil {
		mergeLog.Error("loading defaults for bootstrap", "error", err)
		os.Exit(1)
	}

	// Apply schema migrations (e.g. set version = 1 if unversioned)
	Migrate(cfg)

	if err := saveConfig(configPath, cfg); err != nil {
		mergeLog.Error("bootstrapping config", "error", err)
		os.Exit(1)
	}
	mergeLog.Info("successfully bootstrapped services.yaml", "path", configPath)
}

// saveConfig marshals and saves the Config struct with standard header comments.
func saveConfig(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	header := `# homelab-daemon — service orchestration config
# Generated from NixOS managedServices declarations on activation.
# Edit this file freely to adjust order, delays, and restart policies.
# Restart policies: no | on-failure | unless-stopped | always
# unless-stopped: like always, but remembers user-initiated stops across reboots.

`
	fileData := append([]byte(header), data...)

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, fileData, 0o644); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("commit config: %w", err)
	}
	return nil
}

// copyFile is a helper to copy a file.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// stringSlicesEqual compares two string slices for equality.
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
