// Package collector gathers host statistics (CPU, memory, disk, network,
// processes, GPU) on a fixed interval. Moved from homelab-dash so the
// daemon owns all privileged and /sys-adjacent reads.
package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"

	logging "github.com/ogglord/homelab-logging"
)

var log = logging.Logger("daemon_collector")

// ProcessStat holds top-N process info.
type ProcessStat struct {
	PID    int32   `json:"PID"`
	Name   string  `json:"Name"`
	CPU    float64 `json:"CPU"`
	Memory float32 `json:"Memory"`
}

// DiskStat holds disk usage for a mount point.
type DiskStat struct {
	Mountpoint string  `json:"Mountpoint"`
	Total      uint64  `json:"Total"`
	Used       uint64  `json:"Used"`
	Percent    float64 `json:"Percent"`
	UsedStr    string  `json:"UsedStr"`
	TotalStr   string  `json:"TotalStr"`
}

// HostInfo holds static + dynamic host metadata.
type HostInfo struct {
	Hostname      string `json:"Hostname"`
	OS            string `json:"OS"`
	KernelVersion string `json:"KernelVersion"`
	Uptime        uint64 `json:"Uptime"`
	CPUModel      string `json:"CPUModel"`
	CPUCores      int    `json:"CPUCores"`
	Motherboard   string `json:"Motherboard"`
	Packages      int    `json:"Packages"`
}

// GpuStats holds a single Intel iGPU sample from intel_gpu_top.
type GpuStats struct {
	RenderBusy float64 `json:"RenderBusy"`
	VideoBusy  float64 `json:"VideoBusy"`
	FreqMHz    float64 `json:"FreqMHz"`
	PowerW     float64 `json:"PowerW"`
	RC6Pct     float64 `json:"RC6Pct"`
	Available  bool    `json:"Available"`
	ErrMsg     string  `json:"ErrMsg"`
}

// Stats is the aggregated snapshot returned by Get().
type Stats struct {
	CPUUsage    float64       `json:"CPUUsage"`
	CPUTemp     float64       `json:"CPUTemp"`
	NVMeTemp    float64       `json:"NVMeTemp"`
	MemTotal    uint64        `json:"MemTotal"`
	MemUsed     uint64        `json:"MemUsed"`
	MemPercent  float64       `json:"MemPercent"`
	Disks       []DiskStat    `json:"Disks"`
	NetSentRate float64       `json:"NetSentRate"`
	NetRecvRate float64       `json:"NetRecvRate"`
	TopCPU      []ProcessStat `json:"TopCPU"`
	TopMem      []ProcessStat `json:"TopMem"`
	System      HostInfo      `json:"System"`
	Gpu         GpuStats      `json:"Gpu"`

	// String-formatted versions of computed fields (populated by poll()).
	MemUsedStr  string `json:"MemUsedStr"`
	MemTotalStr string `json:"MemTotalStr"`
	UptimeStr   string `json:"UptimeStr"`
}

// Option configures a Collector.
type Option func(*Collector)

// WithMounts sets the mount points for disk monitoring.
func WithMounts(mounts []string) Option {
	return func(c *Collector) {
		if len(mounts) > 0 {
			c.mounts = mounts
		}
	}
}

// Collector gathers host statistics on a fixed interval.
// Safe for concurrent use after Start() is called.
type Collector struct {
	mu    sync.RWMutex
	stats Stats

	// Network rate tracking.
	lastNetSent  uint64
	lastNetRecv  uint64
	lastPollTime time.Time

	// Mount points for disk monitoring.
	mounts []string

	// Static info computed once.
	staticInfo HostInfo

	// Cached top-N process lists (updated every 10s).
	cachedTopCPU    []ProcessStat
	cachedTopMem    []ProcessStat
	lastProcessPoll time.Time

	// Cached GPU stats (updated every 5s; each sample blocks ~250 ms).
	cachedGpu    GpuStats
	lastGpuPoll  time.Time
	// cachedGPUDevice is the /dev/dri/cardN path for the Intel iGPU,
	// discovered once from sysfs and reused on every poll.
	cachedGPUDevice string
}

// NewCollector creates a Collector with the given mount points.
// Call Start() to begin background polling.
func NewCollector(opts ...Option) *Collector {
	c := &Collector{
		mounts: []string{"/"},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Start begins background polling at the given interval.
// It blocks until ctx is cancelled.
func (c *Collector) Start(ctx context.Context, interval time.Duration) {
	c.staticInfo = c.collectStaticInfo()

	// Seed network counters.
	cpu.Percent(0, false)

	n, _ := net.IOCounters(true)
	for _, iface := range n {
		if isPhysicalInterface(iface.Name) {
			c.lastNetSent += iface.BytesSent
			c.lastNetRecv += iface.BytesRecv
		}
	}

	c.lastPollTime = time.Now()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.mu.Lock()
			c.stats = c.poll()
			c.mu.Unlock()
		}
	}
}

// Get returns the most recent Stats snapshot.
func (c *Collector) Get() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// --- helpers ---

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatUptime(uptime uint64) string {
	days := uptime / 86400
	hours := (uptime % 86400) / 3600
	mins := (uptime % 3600) / 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	return fmt.Sprintf("%dh %dm", hours, mins)
}

func isPhysicalInterface(iface string) bool {
	// Exclude virtual, tunnel, and bridge interfaces.
	switch {
	case iface == "lo":
		return false
	case strings.HasPrefix(iface, "veth"):
		return false
	case strings.HasPrefix(iface, "docker"):
		return false
	case strings.HasPrefix(iface, "br-"):
		return false
	case strings.HasPrefix(iface, "virbr"):
		return false
	case strings.HasPrefix(iface, "tun"):
		return false
	case strings.HasPrefix(iface, "tap"):
		return false
	case strings.HasPrefix(iface, "tailscale"):
		return false
	case strings.HasPrefix(iface, "podman"):
		return false
	case strings.HasPrefix(iface, "cni-"):
		return false
	}
	// Only include Ethernet and WiFi physical interfaces.
	return strings.HasPrefix(iface, "eth") || strings.HasPrefix(iface, "en") ||
		strings.HasPrefix(iface, "wl") || strings.HasPrefix(iface, "ww")
}

func (c *Collector) collectStaticInfo() HostInfo {
	var hi HostInfo

	h, _ := host.Info()
	if h != nil {
		hi.Hostname = h.Hostname
		hi.KernelVersion = h.KernelVersion
	}

	if out, err := exec.Command("nixos-version").Output(); err == nil {
		fullVersion := strings.TrimSpace(string(out))

		parts := strings.Split(fullVersion, ".")
		if len(parts) >= 2 {
			hi.OS = "NixOS " + parts[0] + "." + parts[1]
		} else {
			hi.OS = "NixOS " + fullVersion
		}
	} else if data, err := os.ReadFile("/etc/os-release"); err == nil {
		for line := range strings.SplitSeq(string(data), "\n") {
			if after, ok := strings.CutPrefix(line, "VERSION_ID="); ok {
				hi.OS = "NixOS " + strings.Trim(after, "\"")
				break
			}
		}
	}

	if hi.OS == "" && h != nil {
		hi.OS = h.OS + " " + h.PlatformVersion
	}

	ci, _ := cpu.Info()
	if len(ci) > 0 {
		hi.CPUModel = ci[0].ModelName
	}
	hi.CPUCores, _ = cpu.Counts(true)

	if b, err := os.ReadFile("/sys/class/dmi/id/board_name"); err == nil {
		hi.Motherboard = strings.TrimSpace(string(b))
	} else if b, err := os.ReadFile("/sys/class/dmi/id/board_vendor"); err == nil {
		hi.Motherboard = strings.TrimSpace(string(b))
	}

	if out, err := exec.Command("nix-store", "-qR", "/run/current-system").Output(); err == nil {
		hi.Packages = len(strings.Split(strings.TrimSpace(string(out)), "\n"))
	}

	return hi
}

func (c *Collector) poll() Stats {
	var s Stats

	now := time.Now()
	dt := now.Sub(c.lastPollTime).Seconds()

	pct, _ := cpu.Percent(0, false)
	if len(pct) > 0 {
		s.CPUUsage = pct[0]
	}

	temps, _ := host.SensorsTemperatures()
	for _, t := range temps {
		if t.SensorKey == "coretemp_package_id_0" || t.SensorKey == "k10temp_package_id_0" || t.SensorKey == "coretemp_core_0" {
			s.CPUTemp = t.Temperature
			break
		}
	}

	if s.CPUTemp == 0 && len(temps) > 0 {
		s.CPUTemp = temps[0].Temperature
	}

	// NVMe temperature (SSD).
	for _, t := range temps {
		if strings.HasPrefix(t.SensorKey, "nvme") {
			s.NVMeTemp = t.Temperature
			break
		}
	}

	m, _ := mem.VirtualMemory()
	if m != nil {
		s.MemTotal = m.Total
		s.MemUsed = m.Used
		s.MemPercent = m.UsedPercent
	}

	s.Disks = make([]DiskStat, 0, len(c.mounts))
	for _, mount := range c.mounts {
		usage, err := disk.Usage(mount)
		if err == nil {
			s.Disks = append(s.Disks, DiskStat{
				Mountpoint: mount,
				Total:      usage.Total,
				Used:       usage.Used,
				Percent:    usage.UsedPercent,
			})
		}
	}

	n, _ := net.IOCounters(true)

	var currentNetSent, currentNetRecv uint64

	for _, iface := range n {
		if isPhysicalInterface(iface.Name) {
			currentNetSent += iface.BytesSent
			currentNetRecv += iface.BytesRecv
		}
	}

	if dt > 0 {
		s.NetSentRate = float64(currentNetSent-c.lastNetSent) / dt
		s.NetRecvRate = float64(currentNetRecv-c.lastNetRecv) / dt
	}

	c.lastNetSent = currentNetSent
	c.lastNetRecv = currentNetRecv
	c.lastPollTime = now

	hi, _ := host.Info()

	s.System = c.staticInfo
	if hi != nil {
		s.System.Uptime = hi.Uptime
	}

	// Process polling (throttled to every 3s).
	if now.Sub(c.lastProcessPoll) >= 3*time.Second {
		procs, _ := process.Processes()

		var pStats []ProcessStat

		for _, p := range procs {
			name, err := p.Name()
			if err != nil {
				continue
			}

			cpuP, _ := p.CPUPercent()
			memP, _ := p.MemoryPercent()
			pStats = append(pStats, ProcessStat{
				PID:    p.Pid,
				Name:   name,
				CPU:    cpuP,
				Memory: memP,
			})
		}

		sort.Slice(pStats, func(i, j int) bool { return pStats[i].CPU > pStats[j].CPU })

		if n := min(5, len(pStats)); n > 0 {
			c.cachedTopCPU = make([]ProcessStat, n)
			copy(c.cachedTopCPU, pStats[:n])
		}

		sort.Slice(pStats, func(i, j int) bool { return pStats[i].Memory > pStats[j].Memory })

		if n := min(5, len(pStats)); n > 0 {
			c.cachedTopMem = make([]ProcessStat, n)
			copy(c.cachedTopMem, pStats[:n])
		}

		c.lastProcessPoll = now
	}

	s.TopCPU = c.cachedTopCPU
	s.TopMem = c.cachedTopMem

	// GPU stats (throttled to every 5s; each sample takes ~250 ms).
	if now.Sub(c.lastGpuPoll) >= 5*time.Second {
		// Discover the Intel DRM card once; reuse thereafter.
		if c.cachedGPUDevice == "" {
			c.cachedGPUDevice = findIntelDRMCard()
			if c.cachedGPUDevice != "" {
				log.Info("intel iGPU device", "path", c.cachedGPUDevice)
			} else {
				log.Warn("no Intel iGPU found in /sys/class/drm")
			}
		}
		c.cachedGpu = pollGPU(c.cachedGPUDevice)
		c.lastGpuPoll = now
	}
	s.Gpu = c.cachedGpu

	s.MemUsedStr = formatBytes(s.MemUsed)
	s.MemTotalStr = formatBytes(s.MemTotal)
	s.UptimeStr = formatUptime(s.System.Uptime)
	for i := range s.Disks {
		s.Disks[i].UsedStr = formatBytes(s.Disks[i].Used)
		s.Disks[i].TotalStr = formatBytes(s.Disks[i].Total)
	}

	return s
}

// gpuTopJSON mirrors the fields emitted by `intel_gpu_top -J`.
type gpuTopJSON struct {
	Frequency struct {
		Actual float64 `json:"actual"`
	} `json:"frequency"`
	Engines map[string]struct {
		Busy float64 `json:"busy"`
	} `json:"engines"`
	Power struct {
		GPU float64 `json:"GPU"`
	} `json:"power"`
	RC6 struct {
		Value float64 `json:"value"`
	} `json:"rc6"`
}

// lastGoodGPU caches the last successful read so a transient parse failure
// doesn't blank the widget. Module-level since pollGPU is package-level.
var lastGoodGPU GpuStats

// findIntelDRMCard scans /sys/class/drm for a card owned by Intel (vendor
// 0x8086) and returns its /dev/dri/cardN path, or "" if not found.
func findIntelDRMCard() string {
	entries, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return ""
	}
	for _, e := range entries {
		name := e.Name()
		// Only top-level card entries (skip card1-HDMI-A-1 style subdirs).
		if !strings.HasPrefix(name, "card") || strings.Contains(name, "-") {
			continue
		}
		vendor, err := os.ReadFile("/sys/class/drm/" + name + "/device/vendor")
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(vendor)) == "0x8086" {
			return "/dev/dri/" + name
		}
	}
	return ""
}

// pollGPU reads the JSON snapshot written by the intel-gpu-stats sidecar
// (see modules/intel-gpu-stats.nix). The sidecar runs intel_gpu_top -J -s 1000
// briefly and dumps one or two concatenated JSON objects to a file. We parse
// the last complete object so the dash never has to shell out to a privileged
// tool. Returns the cached last-good value on transient failures to keep the
// widget populated. device is ignored (kept for signature compatibility).
func pollGPU(device string) GpuStats {
	_ = device
	path := os.Getenv("HOMELAB_INTEL_GPU_STATS")
	if path == "" {
		path = "/run/intel-gpu-stats/gpu.json"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if lastGoodGPU.Available {
			return lastGoodGPU
		}
		return GpuStats{ErrMsg: "read " + path + ": " + err.Error()}
	}

	// File may hold one or more JSON objects (possibly wrapped in an array
	// or with stray commas/brackets from intel_gpu_top's odd output).
	// Stream-decode and keep the last successfully parsed value.
	dec := json.NewDecoder(bytes.NewReader(stripJSONNoise(data)))
	var last *gpuTopJSON
	for {
		var raw gpuTopJSON
		err := dec.Decode(&raw)
		if err != nil {
			break
		}
		// Skip empty/zero objects (intel_gpu_top emits a header sample).
		if raw.Frequency.Actual == 0 && raw.Power.GPU == 0 && len(raw.Engines) == 0 {
			continue
		}
		r := raw
		last = &r
	}

	if last == nil {
		if lastGoodGPU.Available {
			return lastGoodGPU
		}
		return GpuStats{ErrMsg: "no valid JSON record in " + path}
	}

	g := GpuStats{
		FreqMHz:   last.Frequency.Actual,
		PowerW:    last.Power.GPU,
		RC6Pct:    last.RC6.Value,
		Available: true,
	}
	if e, ok := last.Engines["Render/3D"]; ok {
		g.RenderBusy = e.Busy
	}
	if e, ok := last.Engines["Video"]; ok {
		g.VideoBusy = e.Busy
	}
	lastGoodGPU = g
	return g
}

// stripJSONNoise removes wrapping brackets and stray commas between objects
// so that json.Decoder can stream-decode the records intel_gpu_top emits.
func stripJSONNoise(b []byte) []byte {
	s := strings.TrimSpace(string(b))
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	// Replace "},\n{" object separators with "}\n{" so Decoder sees a
	// stream of objects rather than a malformed array.
	s = strings.ReplaceAll(s, "},\n{", "}\n{")
	s = strings.ReplaceAll(s, "}, {", "} {")
	return []byte(s)
}
