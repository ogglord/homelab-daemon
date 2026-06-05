package bcachefs

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	logging "github.com/ogglord/homelab-logging"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/ogglord/homelab-daemon/internal/cmdrunner"
)

var log = logging.Logger("storage")

type DiskIOStats struct {
	DeviceName   string `json:"device_name"`
	ReadIOs      uint64 `json:"read_ios"`       // completed reads
	WriteIOs     uint64 `json:"write_ios"`      // completed writes
	ReadSectors  uint64 `json:"read_sectors"`   // sectors read
	WriteSectors uint64 `json:"write_sectors"`  // sectors written
	IOInProgress uint64 `json:"io_in_progress"` // currently in-flight I/Os
}

type PoolUsage struct {
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

type Disk struct {
	Name           string       `json:"name"`
	Path           string       `json:"path"`
	Size           uint64       `json:"size"`
	Type           string       `json:"type"`
	Model          string       `json:"model"`
	Serial         string       `json:"serial"`
	FSType         string       `json:"fstype"`
	Mountpoint     string       `json:"mountpoint"`
	Label          string       `json:"label"`
	HasChildren    bool         `json:"has_children"`
	BcachefsLabel  string       `json:"bcachefs_label"`  // NEW — e.g. "ssd.ssd1", "hdd.hdd1"
	DataTarget     string       `json:"data_target"`     // NEW — "foreground", "background", or ""
	MetadataTarget string       `json:"metadata_target"` // NEW — "foreground", "background", or ""
	IO             *DiskIOStats `json:"io,omitempty"`
	FriendlyName   string       `json:"friendly_name"` // NEW — friendly display name via smartctl
}

type Pool struct {
	UUID             string     `json:"uuid"`
	Name             string     `json:"name"`  // NEW — user-set or label fallback
	State            string     `json:"state"` // mounted, unmounted
	Mountdir         string     `json:"mountdir"`
	Label            string     `json:"label"`             // NEW — from bcachefs show-super
	DataReplicas     int        `json:"data_replicas"`     // NEW — e.g. 1
	MetadataReplicas int        `json:"metadata_replicas"` // NEW — e.g. 2
	Disks            []Disk     `json:"disks"`
	Usage            *PoolUsage `json:"usage,omitempty"` // NEW — only when mounted
}

type Subvolume struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	UsedBytes int64  `json:"used_bytes"` // NEW — -1 if not yet calculated
}

// runCommand runs a command as root (daemon is already root).
func runCommand(cmdName string, args ...string) (string, error) {
	b := cmdrunner.New("storage", cmdName, args...)
	res, err := b.Run()
	return res.Stdout, err
}

// lsblkOutput represents the JSON output from lsblk.
type lsblkOutput struct {
	Blockdevices []lsblkDevice `json:"blockdevices"`
}

type lsblkDevice struct {
	Name        string        `json:"name"`
	Path        string        `json:"path"`
	Size        interface{}   `json:"size"` // Sometimes a string or int depending on lsblk version, we requested bytes (-b) so it should be int or string representation of int
	Type        string        `json:"type"`
	Model       string        `json:"model"`
	Serial      string        `json:"serial"`
	FSType      string        `json:"fstype"`
	Mountpoint  string        `json:"mountpoint"`
	Mountpoints []string      `json:"mountpoints"` // Newer lsblk uses an array
	Label       string        `json:"label"`
	Children    []lsblkDevice `json:"children"`
}

// GetDisks returns all block devices using lsblk.
func GetDisks() ([]Disk, error) {
	out, err := runCommand("lsblk", "-J", "-b", "-o", "NAME,PATH,SIZE,TYPE,MODEL,SERIAL,FSTYPE,MOUNTPOINT,LABEL")
	if err != nil {
		return nil, err
	}

	var output lsblkOutput
	if err := json.Unmarshal([]byte(out), &output); err != nil {
		return nil, fmt.Errorf("failed to parse lsblk output: %w", err)
	}

	var disks []Disk
	var extractDisks func([]lsblkDevice)
	extractDisks = func(devices []lsblkDevice) {
		for _, dev := range devices {
			// Convert size
			var size uint64
			switch v := dev.Size.(type) {
			case float64:
				size = uint64(v)
			case string:
				fmt.Sscanf(v, "%d", &size)
			}

			mountpoint := dev.Mountpoint
			if mountpoint == "" && len(dev.Mountpoints) > 0 {
				mountpoint = dev.Mountpoints[0]
			}

			friendly := getCachedFriendlyDiskName(dev.Path, strings.TrimSpace(dev.Model), size)
			disks = append(disks, Disk{
				Name:         dev.Name,
				Path:         dev.Path,
				Size:         size,
				Type:         dev.Type,
				Model:        strings.TrimSpace(dev.Model),
				Serial:       strings.TrimSpace(dev.Serial),
				FSType:       dev.FSType,
				Mountpoint:   mountpoint,
				Label:        dev.Label,
				HasChildren:  len(dev.Children) > 0,
				FriendlyName: friendly,
			})
			if len(dev.Children) > 0 {
				extractDisks(dev.Children)
			}
		}
	}
	extractDisks(output.Blockdevices)
	return disks, nil
}

// findBcachefsMount returns the canonical bcachefs mountpoint for a device,
// reading /proc/mounts authoritatively. bcachefs entries list their
// underlying devices joined by colons (e.g. /dev/sda:/dev/sdb /pool bcachefs).
// lsblk's `mountpoint` field is alphabetical and can return bind/autofs
// overlays (filebrowser's /srv/browse/pool) instead of the real /pool.
// Returns "" if the device isn't part of any mounted bcachefs pool.
func findBcachefsMount(devicePath string) string {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[2] != "bcachefs" {
			continue
		}
		for _, d := range strings.Split(fields[0], ":") {
			if d == devicePath {
				return fields[1]
			}
		}
	}
	return ""
}

// DiscoverPools finds bcachefs pools by grouping disks with fstype="bcachefs".
// We group them by their UUID using `bcachefs show-super`.
func DiscoverPools() ([]Pool, error) {
	log.Info("Discovering bcachefs pools...")
	disks, err := GetDisks()
	if err != nil {
		log.Error("failed to get disks during discovery", "error", err)
		return nil, err
	}

	// Group bcachefs disks by UUID.
	// show-super for any disk in a pool returns info for all members, so we
	// cache the parsed result and reuse it for sibling disks in the same pool.
	poolMap := make(map[string]*Pool)
	// Cache the parsed superblock after the first show-super call.
	// A single show-super returns device info for all members of the array
	// (all 4 disks share one External UUID), so sibling disks can skip the
	// CLI invocation entirely.
	var super ParsedSuper
	var superValid bool

	for _, d := range disks {
		// Sometimes lsblk doesn't identify the FSType of an offline bcachefs device.
		// We should probe it if it's explicitly bcachefs OR if it has no fstype/mountpoint.
		if d.FSType != "bcachefs" && d.FSType != "" {
			continue
		}

		if superValid {
			log.Debug("reusing cached super block for sibling device", "device", d.Path)
		} else {
			out, err := runCommand("bcachefs", "show-super", d.Path)
			if err != nil {
				log.Warn("bcachefs show-super failed (likely not a bcachefs device)", "device", d.Path, "error", err)
				continue
			}
			super = parseSuper(out)
			if super.UUID == "" {
				log.Warn("Could not extract External UUID from output", "device", d.Path)
				continue
			}
			superValid = true
		}

		log.Info("found bcachefs device", "device", d.Path, "uuid", super.UUID, "mountpoint", d.Mountpoint)

		p, exists := poolMap[super.UUID]
		if !exists {
			p = &Pool{
				UUID:             super.UUID,
				State:            "unmounted",
				Label:            super.Label,
				DataReplicas:     super.DataReplicas,
				MetadataReplicas: super.MetadataReplicas,
			}
			poolMap[super.UUID] = p
		}

		for _, dev := range super.Devices {
			if d.Path == dev.Path || (dev.Path != "" && strings.HasSuffix(d.Path, dev.Path)) || (d.Name != "" && strings.HasSuffix(dev.Path, d.Name)) {
				d.BcachefsLabel = dev.Label
				d.DataTarget, d.MetadataTarget = determineDiskTargets(dev.Label, super.ForegroundTarget, super.MetadataTarget, super.BackgroundTarget)
				break
			}
		}

		// Prefer the canonical bcachefs mountpoint from /proc/mounts. lsblk's
		// `mountpoint` field is alphabetical and can return bind/autofs
		// overlays (e.g. filebrowser's /srv/browse/pool) instead of the real
		// /pool. Fall back to whatever lsblk reported if /proc/mounts has
		// nothing.
		if canon := findBcachefsMount(d.Path); canon != "" {
			p.State = "mounted"
			p.Mountdir = canon
		} else if d.Mountpoint != "" {
			p.State = "mounted"
			p.Mountdir = d.Mountpoint
		}

		p.Disks = append(p.Disks, d)
	}

	var pools []Pool
	for _, p := range poolMap {
		pools = append(pools, *p)
	}
	return pools, nil
}

type ParsedSuper struct {
	UUID             string
	Label            string
	DataReplicas     int
	MetadataReplicas int
	ForegroundTarget string
	MetadataTarget   string
	BackgroundTarget string
	Devices          []ParsedDevice
}

type ParsedDevice struct {
	Index string
	Path  string
	Model string
	Label string
}

func parseSuper(output string) ParsedSuper {
	var super ParsedSuper
	lines := strings.Split(output, "\n")

	super.ForegroundTarget = ""
	super.MetadataTarget = ""
	super.BackgroundTarget = ""

	var currentDevice *ParsedDevice

	for _, line := range lines {
		if strings.Contains(line, "Device ") && strings.Contains(line, ":") && !strings.Contains(line, "Device index:") {
			idx := strings.Index(line, "Device ")
			colonIdx := strings.Index(line[idx:], ":")
			if colonIdx != -1 {
				colonIdx += idx
				devNum := strings.TrimSpace(line[idx+7 : colonIdx])

				rest := line[colonIdx+1:]
				fields := strings.Fields(rest)
				path := ""
				model := ""
				if len(fields) > 0 {
					path = fields[0]
				}
				if len(fields) > 1 {
					model = fields[1]
				}

				var dev ParsedDevice
				dev.Index = devNum
				dev.Path = path
				dev.Model = model
				super.Devices = append(super.Devices, dev)
				currentDevice = &super.Devices[len(super.Devices)-1]
			}
		}

		if currentDevice != nil {
			if strings.Contains(line, "Label:") {
				parts := strings.SplitN(line, "Label:", 2)
				if len(parts) == 2 {
					lbl := strings.TrimSpace(parts[1])
					fields := strings.Fields(lbl)
					if len(fields) > 0 {
						currentDevice.Label = fields[0]
						if currentDevice.Label == "(none)" {
							currentDevice.Label = ""
						}
					}
				}
			}
		} else {
			if strings.Contains(line, "External UUID:") {
				parts := strings.SplitN(line, "External UUID:", 2)
				if len(parts) == 2 {
					val := strings.TrimSpace(parts[1])
					fields := strings.Fields(val)
					if len(fields) > 0 {
						super.UUID = fields[0]
					}
				}
			}
			if strings.Contains(line, "Label:") && !strings.Contains(line, "Device ") {
				parts := strings.SplitN(line, "Label:", 2)
				if len(parts) == 2 {
					val := strings.TrimSpace(parts[1])
					fields := strings.Fields(val)
					if len(fields) > 0 {
						super.Label = fields[0]
						if super.Label == "(none)" {
							super.Label = ""
						}
					}
				}
			}
			if strings.Contains(line, "data_replicas:") {
				parts := strings.SplitN(line, "data_replicas:", 2)
				if len(parts) == 2 {
					val := strings.TrimSpace(parts[1])
					fields := strings.Fields(val)
					if len(fields) > 0 {
						fmt.Sscanf(fields[0], "%d", &super.DataReplicas)
					}
				}
			}
			if strings.Contains(line, "metadata_replicas:") {
				parts := strings.SplitN(line, "metadata_replicas:", 2)
				if len(parts) == 2 {
					val := strings.TrimSpace(parts[1])
					fields := strings.Fields(val)
					if len(fields) > 0 {
						fmt.Sscanf(fields[0], "%d", &super.MetadataReplicas)
					}
				}
			}
			if strings.Contains(line, "foreground_target:") {
				parts := strings.SplitN(line, "foreground_target:", 2)
				if len(parts) == 2 {
					val := strings.TrimSpace(parts[1])
					fields := strings.Fields(val)
					if len(fields) > 0 {
						super.ForegroundTarget = fields[0]
					}
				}
			}
			if strings.Contains(line, "metadata_target:") {
				parts := strings.SplitN(line, "metadata_target:", 2)
				if len(parts) == 2 {
					val := strings.TrimSpace(parts[1])
					fields := strings.Fields(val)
					if len(fields) > 0 {
						super.MetadataTarget = fields[0]
					}
				}
			}
			if strings.Contains(line, "background_target:") {
				parts := strings.SplitN(line, "background_target:", 2)
				if len(parts) == 2 {
					val := strings.TrimSpace(parts[1])
					fields := strings.Fields(val)
					if len(fields) > 0 {
						super.BackgroundTarget = fields[0]
					}
				}
			}
		}
	}
	return super
}

func determineDiskTargets(diskLabel, fgTarget, metaTarget, bgTarget string) (string, string) {
	dataTarget := ""
	metadataTarget := ""

	if diskLabel != "" {
		group := diskLabel
		if idx := strings.Index(diskLabel, "."); idx != -1 {
			group = diskLabel[:idx]
		}

		if fgTarget != "" && group == fgTarget {
			dataTarget = "foreground"
		} else if bgTarget != "" && group == bgTarget {
			dataTarget = "background"
		} else if fgTarget == "ssd" && group == "hdd" {
			dataTarget = "background"
		}

		if metaTarget != "" && group == metaTarget {
			metadataTarget = "foreground"
		} else if fgTarget == "ssd" && group == "hdd" {
			metadataTarget = "background"
		}
	}

	return dataTarget, metadataTarget
}

func extractField(output, prefix string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, prefix) {
			parts := strings.SplitN(line, prefix, 2)
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				if idx := strings.Index(val, "\x1b"); idx != -1 {
					val = val[:idx]
				}
				return strings.TrimSpace(val)
			}
		}
	}
	return ""
}

// GetUnassignedDisks returns disks that are not part of any filesystem and not partitions of mounted disks.
func GetUnassignedDisks() ([]Disk, error) {
	log.Info("Scanning for unassigned disks...")
	disks, err := GetDisks()
	if err != nil {
		return nil, err
	}

	var unassigned []Disk
	for _, d := range disks {
		if d.Type != "disk" {
			continue
		}
		if d.HasChildren {
			continue
		}
		if d.FSType != "" || d.Mountpoint != "" {
			continue
		}
		out, err := runCommand("bcachefs", "show-super", d.Path)
		if err == nil && extractField(out, "External UUID:") != "" {
			log.Debug("disk belongs to bcachefs pool, not unassigned", "device", d.Path)
			continue
		} else if err != nil {
			log.Debug("unassigned disk check bcachefs probe failed (expected for empty disks)", "device", d.Path, "error", err)
		}

		log.Info("found unassigned disk", "device", d.Path, "size", d.Size)
		unassigned = append(unassigned, d)
	}
	log.Info("unassigned disks scan complete", "count", len(unassigned))
	return unassigned, nil
}

// Mount mounts a bcachefs pool by passing all its devices to the mount command.
func Mount(devices []string, mountpoint string) error {
	log.Info("Mounting bcachefs pool", "devices", devices, "mountpoint", mountpoint)
	if len(devices) == 0 {
		log.Error("mount failed", "reason", "no devices provided")
		return errors.New("no devices provided")
	}
	devStr := strings.Join(devices, ":")
	_, err := runCommand("mount", "-t", "bcachefs", devStr, mountpoint)
	if err != nil {
		log.Error("mount failed", "error", err)
	} else {
		log.Info("mount successful", "mountpoint", mountpoint)
	}
	return err
}

// Unmount unmounts a bcachefs pool.
func Unmount(mountpoint string) error {
	log.Info("Unmounting bcachefs pool", "mountpoint", mountpoint)
	_, err := runCommand("umount", mountpoint)
	if err != nil {
		log.Error("unmount failed", "error", err)
	} else {
		log.Info("unmount successful", "mountpoint", mountpoint)
	}
	return err
}

// ListSubvolumes lists subvolumes for a given mountpoint.
// bcachefs subvolume list output format:
//
//	Path                     ID       Created          Flags        Size
//	test2                    2        2026-05-27 21:46 -
//
// Paths are relative to the mountpoint; we prepend it to return absolute paths.
func ListSubvolumes(mountpoint string) ([]Subvolume, error) {
	log.Info("Listing bcachefs subvolumes", "mountpoint", mountpoint)
	out, err := runCommand("bcachefs", "subvolume", "list", mountpoint)
	if err != nil {
		log.Error("failed to list subvolumes", "error", err)
		return nil, err
	}

	var subvolumes []Subvolume
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Subvolumes") || strings.HasPrefix(line, "Path") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		relPath := parts[0]
		absPath := filepath.Join(mountpoint, relPath)
		subvolumes = append(subvolumes, Subvolume{
			ID:        parts[1],
			Path:      absPath,
			UsedBytes: -1,
		})
	}
	return subvolumes, nil
}

// CreateSubvolume creates a subvolume at the given path.
func CreateSubvolume(path string) error {
	log.Info("Creating bcachefs subvolume", "path", path)
	_, err := runCommand("bcachefs", "subvolume", "create", path)
	if err != nil {
		log.Error("failed to create subvolume", "error", err)
	}
	return err
}

// ErrSubvolumeNotEmpty is returned by DeleteSubvolume when the target still
// contains files. The caller (UI/API) is expected to surface this so the user
// can clean the contents out first; we never silently destroy data.
var ErrSubvolumeNotEmpty = errors.New("subvolume is not empty")

// DeleteSubvolume deletes a subvolume at the given path. Refuses to proceed
// if the subvolume still contains entries — the user must clean them out
// first via their own filesystem tools.
func DeleteSubvolume(path string) error {
	log.Info("Deleting bcachefs subvolume", "path", path)

	entries, err := os.ReadDir(path)
	if err == nil && len(entries) > 0 {
		log.Warn("refusing to delete non-empty subvolume", "path", path, "entries", len(entries))
		return fmt.Errorf("%w: %d entries remain at %s — remove its contents first", ErrSubvolumeNotEmpty, len(entries), path)
	}

	_, err = runCommand("bcachefs", "subvolume", "delete", path)
	if err != nil {
		log.Error("failed to delete subvolume", "error", err)
	}
	return err
}

// InitFolders ensures the canonical /pool layout exists with hardened, per-
// folder ownership. Idempotent: missing entries are created, existing entries
// are left in place (never renames or removes anything). Ownership and mode
// are reapplied recursively every run so each subtree converges back to:
//
//	<mountpoint>/                       root:media  0751  (media can ls,
//	                                                       others traverse-only)
//	├── media/      [subvol]            root:media  2770
//	│   ├── library/                    root:media  2770
//	│   │   ├── movies/                 root:media  2770
//	│   │   ├── shows/                  root:media  2770
//	│   │   ├── music/                  root:media  2770
//	│   │   ├── books/                  root:media  2770
//	│   │   └── audiobooks/             root:media  2770
//	│   ├── qbittorrent/                root:media  2770  (completed dl)
//	│   └── .incomplete/                root:media  2770  (in-progress dl)
//	├── immich/     [subvol]            root:immich 2770
//	├── tmp/        [subvol]            root:users  2770  (ogge's primary)
//	└── backups/    [subvol]            root:root   0700  (root-only)
//
// The mountpoint is 0711 (traverse but not list) because not every consumer
// shares a group — immich is in `immich` group, the arr stack is in `media`,
// ogge's scratch is in `users`. Each subdir handles its own ACL via setgid,
// so children inherit the right group automatically.
//
// The media subtree mirrors nixarr's tmpfiles layout (sonarr/radarr/plex
// modules). We own it from Init rather than letting tmpfiles drift between
// 2775 (sonarr/radarr) and 0775 (plex) on every reboot. systemd-tmpfiles
// `d` rules are no-ops on existing directories (per tmpfiles.d spec), so
// our perms stick once Init has created the tree.
//
// qBittorrent runs as ogge:media (1000:169) inside its VPN container and is
// a member of the media group, so 2770 lets it write to qbittorrent/ and
// .incomplete/ without the 0777 hack nixarr uses for arbitrary download-
// client UIDs. After Init, point qBittorrent's DefaultSavePath at
// /pool/media/qbittorrent and (optionally) TempPath at /pool/media/.incomplete.
//
// Files inside group-shared subtrees get 0660; backups gets 0600.
func InitFolders(mountpoint string) error {
	log.Info("Initializing /pool layout", "mountpoint", mountpoint)

	// Mountpoint root: 0751 root:media. Members of the `media` group (ogge,
	// the arr stack, plex, jellyfin, filestash) can `ls /pool` to see the
	// layout; everyone else can still traverse (--x) to reach their own
	// subtree, so immich (not in media) can still cd into /pool/immich.
	// Reapplied every Init because bcachefs may hand it out as 0755 at
	// mount time.
	mediaGid, err := lookupGid("media")
	if err != nil {
		return fmt.Errorf("media group not found: %w", err)
	}
	if err := os.Chown(mountpoint, 0, mediaGid); err != nil {
		return fmt.Errorf("chown %s: %w", mountpoint, err)
	}
	if err := os.Chmod(mountpoint, 0o0751); err != nil {
		return fmt.Errorf("chmod %s: %w", mountpoint, err)
	}

	type entry struct {
		path        string      // relative to mountpoint
		isSubvolume bool        // create with `bcachefs subvolume create` vs mkdir
		group       string      // group name to chown to ("root" → 0)
		dirMode     os.FileMode // mode for directories
		fileMode    os.FileMode // mode for files
	}
	entries := []entry{
		// Top-level subvolumes.
		{"media", true, "media", 0o2770, 0o660},
		{"immich", true, "immich", 0o2770, 0o660},
		{"tmp", true, "users", 0o2770, 0o660}, // ogge's primary group
		{"backups", true, "root", 0o0700, 0o600},

		// Media library (mirrors nixarr's plex/sonarr/radarr layout).
		{"media/library", false, "media", 0o2770, 0o660},
		{"media/library/movies", false, "media", 0o2770, 0o660},
		{"media/library/shows", false, "media", 0o2770, 0o660},
		{"media/library/music", false, "media", 0o2770, 0o660},
		{"media/library/books", false, "media", 0o2770, 0o660},
		{"media/library/audiobooks", false, "media", 0o2770, 0o660},

		// qBittorrent target dirs.
		{"media/qbittorrent", false, "media", 0o2770, 0o660},
		{"media/.incomplete", false, "media", 0o2770, 0o660},
	}

	for _, e := range entries {
		gid, err := lookupGid(e.group)
		if err != nil {
			return fmt.Errorf("group %q not found: %w", e.group, err)
		}

		full := filepath.Join(mountpoint, e.path)
		if _, err := os.Stat(full); errors.Is(err, fs.ErrNotExist) {
			if e.isSubvolume {
				if _, err := runCommand("bcachefs", "subvolume", "create", full); err != nil {
					return fmt.Errorf("create subvolume %s: %w", full, err)
				}
			} else {
				if err := os.MkdirAll(full, e.dirMode); err != nil {
					return fmt.Errorf("mkdir %s: %w", full, err)
				}
			}
		} else if err != nil {
			return fmt.Errorf("stat %s: %w", full, err)
		}

		if err := applyTreePerms(full, gid, e.dirMode, e.fileMode); err != nil {
			return err
		}
	}

	// bcachefs fsck creates lost+found at the mountpoint root when it finds
	// orphaned inodes. Drop it if it's empty — fsck will recreate only when
	// it actually has something to put there. A non-empty lost+found means
	// recovered user data, which we never delete automatically.
	lostFound := filepath.Join(mountpoint, "lost+found")
	if items, err := os.ReadDir(lostFound); err == nil {
		if len(items) == 0 {
			if err := os.Remove(lostFound); err != nil {
				log.Warn("failed to remove empty lost+found", "path", lostFound, "error", err)
			} else {
				log.Info("removed empty lost+found", "path", lostFound)
			}
		} else {
			log.Warn("lost+found is not empty, leaving it alone — review recovered files manually", "path", lostFound, "entries", len(items))
		}
	}

	return nil
}

// applyTreePerms walks root and applies root:<gid> + per-type mode to every
// entry. Idempotent. Symlinks are skipped so an in-pool symlink can't redirect
// chown/chmod outside the pool.
func applyTreePerms(root string, gid int, dirMode, fileMode os.FileMode) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if err := os.Chown(p, 0, gid); err != nil {
			return fmt.Errorf("chown %s: %w", p, err)
		}
		mode := fileMode
		if d.IsDir() {
			mode = dirMode
		}
		if err := os.Chmod(p, mode); err != nil {
			return fmt.Errorf("chmod %s: %w", p, err)
		}
		return nil
	})
}

func lookupGid(group string) (int, error) {
	if group == "root" {
		return 0, nil
	}
	g, err := user.LookupGroup(group)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(g.Gid)
}

// GetPoolUsage returns usage stats for a mounted filesystem.
func GetPoolUsage(mountdir string) (*PoolUsage, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(mountdir, &stat)
	if err != nil {
		return nil, err
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	avail := stat.Bavail * uint64(stat.Bsize)
	used := total - free

	var usedPercent float64
	if total > 0 {
		usedPercent = float64(used) / float64(total) * 100.0
	}

	return &PoolUsage{
		TotalBytes:     total,
		UsedBytes:      used,
		AvailableBytes: avail,
		UsedPercent:    usedPercent,
	}, nil
}

// GetDiskIO reads /proc/diskstats and filters by given device names.
func GetDiskIO(deviceNames []string) (map[string]DiskIOStats, error) {
	file, err := os.Open("/proc/diskstats")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	devMap := make(map[string]bool)
	for _, name := range deviceNames {
		devMap[name] = true
	}

	stats := make(map[string]DiskIOStats)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}
		devName := fields[2]
		if !devMap[devName] {
			continue
		}

		readIOs, _ := strconv.ParseUint(fields[3], 10, 64)
		readSectors, _ := strconv.ParseUint(fields[5], 10, 64)
		writeIOs, _ := strconv.ParseUint(fields[7], 10, 64)
		writeSectors, _ := strconv.ParseUint(fields[9], 10, 64)
		ioInProgress, _ := strconv.ParseUint(fields[11], 10, 64)

		stats[devName] = DiskIOStats{
			DeviceName:   devName,
			ReadIOs:      readIOs,
			WriteIOs:     writeIOs,
			ReadSectors:  readSectors,
			WriteSectors: writeSectors,
			IOInProgress: ioInProgress,
		}
	}

	return stats, nil
}

// GetSubvolumeUsage shells out to `du -sb --one-file-system <path>`.
func GetSubvolumeUsage(path string) (int64, error) {
	log.Info("Getting subvolume usage", "path", path)
	out, err := runCommand("du", "-sb", "--one-file-system", path)
	if err != nil {
		return -1, err
	}

	fields := strings.Fields(out)
	if len(fields) < 1 {
		return -1, fmt.Errorf("unexpected du output: %q", out)
	}

	size, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return -1, fmt.Errorf("failed to parse du size: %w", err)
	}

	return size, nil
}

// smartctlJSON represents the structured JSON output from smartctl -j -i.
type smartctlJSON struct {
	ModelFamily  string `json:"model_family"`
	ModelName    string `json:"model_name"`
	UserCapacity struct {
		Bytes uint64 `json:"bytes"`
	} `json:"user_capacity"`
}

var (
	friendlyNameCache   = make(map[string]string)
	friendlyNameCacheMu sync.RWMutex
)

// getCachedFriendlyDiskName resolves a clean display name using a local thread-safe cache.
func getCachedFriendlyDiskName(path string, model string, bytes uint64) string {
	if path == "" {
		return model
	}

	friendlyNameCacheMu.RLock()
	name, exists := friendlyNameCache[path]
	friendlyNameCacheMu.RUnlock()

	if exists {
		return name
	}

	friendlyNameCacheMu.Lock()
	// Double check
	if name, exists = friendlyNameCache[path]; exists {
		friendlyNameCacheMu.Unlock()
		return name
	}

	name = getFriendlyDiskName(path, model, bytes)
	friendlyNameCache[path] = name
	friendlyNameCacheMu.Unlock()

	return name
}

// getFriendlyDiskName runs smartctl -j -i and formats a friendly commercial disk name.
// Capacity is intentionally omitted — it's already shown in the table's Capacity column.
func getFriendlyDiskName(path string, defaultModel string, _ uint64) string {
	out, err := runCommand("smartctl", "-j", "-i", path)
	if err != nil {
		log.Warn("smartctl query failed", "device", path, "error", err)
		return defaultModel
	}

	var info smartctlJSON
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		log.Warn("failed to parse smartctl json", "device", path, "error", err)
		return defaultModel
	}

	modelName := strings.TrimSpace(info.ModelName)
	family := strings.TrimSpace(info.ModelFamily)

	if modelName == "" {
		modelName = defaultModel
	}

	// If modelName already contains a brand (has spaces), use it as-is
	// (e.g. "Samsung SSD 850 PRO 1TB").
	if strings.Contains(info.ModelName, " ") {
		return info.ModelName
	}

	// Strip trailing revision codes like "-2DR166" or "-EXM02B6Q".
	if idx := strings.Index(modelName, "-"); idx != -1 && len(modelName[idx:]) > 4 {
		modelName = modelName[:idx]
	}

	if family != "" {
		return fmt.Sprintf("%s %s", family, modelName)
	}
	return modelName
}
