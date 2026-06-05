package api

// DiskIOStats are per-device I/O counters from /proc/diskstats.
type DiskIOStats struct {
	DeviceName   string `json:"device_name"`
	ReadIOs      uint64 `json:"read_ios"`       // completed reads
	WriteIOs     uint64 `json:"write_ios"`      // completed writes
	ReadSectors  uint64 `json:"read_sectors"`   // sectors read
	WriteSectors uint64 `json:"write_sectors"`  // sectors written
	IOInProgress uint64 `json:"io_in_progress"` // currently in-flight I/Os
}

// PoolUsage is reported by `bcachefs fs usage`.
type PoolUsage struct {
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

// Disk is one physical device inside a Pool (or in Unassigned).
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
	HasChildren    bool         `json:"has_children,omitempty"`
	BcachefsLabel  string       `json:"bcachefs_label"`
	DataTarget     string       `json:"data_target"`
	MetadataTarget string       `json:"metadata_target"`
	IO             *DiskIOStats `json:"io,omitempty"`
	FriendlyName   string       `json:"friendly_name"`
}

// Pool is one bcachefs pool with its member disks.
type Pool struct {
	UUID             string     `json:"uuid"`
	Name             string     `json:"name"`
	State            string     `json:"state"` // "mounted" | "unmounted"
	Mountdir         string     `json:"mountdir"`
	Label            string     `json:"label"`
	DataReplicas     int        `json:"data_replicas"`
	MetadataReplicas int        `json:"metadata_replicas"`
	Disks            []Disk     `json:"disks"`
	Usage            *PoolUsage `json:"usage,omitempty"`
}

// Subvolume is one bcachefs subvolume below a mounted Pool.
type Subvolume struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	UsedBytes int64  `json:"used_bytes"` // -1 if not yet calculated
}

// StorageStatus is the body of GET /api/v1/storage.
type StorageStatus struct {
	Pools      []Pool      `json:"pools"`
	Unassigned []Disk      `json:"unassigned"`
	Subvolumes []Subvolume `json:"subvolumes"`
}

// StoragePoolConfig is the user-managed config (auto-mount etc.) for
// one pool, keyed by UUID. Serialised in services.yaml under storage.pools.
type StoragePoolConfig struct {
	UUID       string `json:"uuid" yaml:"uuid"`
	Mountpoint string `json:"mountpoint" yaml:"mountpoint"`
	Options    string `json:"options" yaml:"options"`
	AutoMount  bool   `json:"auto_mount" yaml:"auto_mount"`
	Name       string `json:"name,omitempty" yaml:"name,omitempty"`
}

// MountRequest — POST /api/v1/storage/mount.
type MountRequest struct {
	Devices    []string `json:"devices"`
	Mountpoint string   `json:"mountpoint"`
}

// UnmountRequest — POST /api/v1/storage/unmount.
type UnmountRequest struct {
	Mountpoint string `json:"mountpoint"`
}

// CheckRequest — POST /api/v1/storage/check.
type CheckRequest struct {
	Devices  []string `json:"devices"`
	Mountdir string   `json:"mountdir"`
	Fix      bool     `json:"fix"`
}

// BalanceRequest — POST /api/v1/storage/balance.
type BalanceRequest struct {
	Mountdir string `json:"mountdir"`
}

// SubvolumeUsageRequest — POST /api/v1/storage/subvolume-usage.
type SubvolumeUsageRequest struct {
	Paths []string `json:"paths"`
}

// SubvolumeUsageResponse is the body of POST /api/v1/storage/subvolume-usage.
type SubvolumeUsageResponse struct {
	Success bool             `json:"success"`
	Usage   map[string]int64 `json:"usage"`
}

// StorageConfigPatch — PATCH /api/v1/storage/config.
type StorageConfigPatch struct {
	Pools []StoragePoolConfig `json:"pools"`
}

// SubvolumeRequest — POST /api/v1/storage/subvolume.
type SubvolumeRequest struct {
	Path string `json:"path"`
}

// InitFoldersRequest — POST /api/v1/storage/init-folders.
type InitFoldersRequest struct {
	Mountpoint string `json:"mountpoint"`
}

// CommandResult is the envelope returned by the storage commands that
// also surface their stdout/stderr (check, balance).
type CommandResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Output  string `json:"output"`
}

// UnmountConflict is returned with HTTP 409 when active services
// depend on a mountpoint that the user asked to unmount.
type UnmountConflict struct {
	Error      string   `json:"error"`
	Message    string   `json:"message"`
	ActiveDeps []string `json:"active_deps"`
}
