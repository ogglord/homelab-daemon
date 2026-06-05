package api

// StatsSnapshot is a point-in-time host statistics snapshot emitted by the
// daemon's stats collector and consumed by the dashboard. Every field uses
// the same JSON tags as the dash's former collector.Stats so the frontend
// contract is preserved.
type StatsSnapshot struct {
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
	MemUsedStr  string        `json:"MemUsedStr"`
	MemTotalStr string        `json:"MemTotalStr"`
	UptimeStr   string        `json:"UptimeStr"`
}

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

// GpuStats holds a single Intel iGPU sample.
type GpuStats struct {
	RenderBusy float64 `json:"RenderBusy"`
	VideoBusy  float64 `json:"VideoBusy"`
	FreqMHz    float64 `json:"FreqMHz"`
	PowerW     float64 `json:"PowerW"`
	RC6Pct     float64 `json:"RC6Pct"`
	Available  bool    `json:"Available"`
	ErrMsg     string  `json:"ErrMsg"`
}
