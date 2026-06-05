package collector

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsPhysicalInterface(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		// Physical interfaces.
		{"eth0", true},
		{"enp3s0", true},
		{"wlan0", true},
		{"wlx1234", true},
		// Virtual / excluded.
		{"lo", false},
		{"veth12345", false},
		{"docker0", false},
		{"br-abc123", false},
		{"virbr0", false},
		{"tun0", false},
		{"tap0", false},
		{"tailscale0", false},
		{"podman0", false},
		{"cni-podman0", false},
		// Edge cases.
		{"", false},
		{"zlo", false},   // doesn't start with en/wl
		{"pci0", false},  // not en/wl, used to slip through old 'p' prefix
		{"f0", false},    // not en/wl, used to slip through old 'f' prefix
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPhysicalInterface(tt.name); got != tt.want {
				t.Errorf("isPhysicalInterface(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// TestPollGPU_FromSidecarFile exercises the new file-backed GPU reader.
// Writes one record matching the format intel_gpu_top -J emits, then a
// second one (concatenated, no array wrapper) so we verify the streaming
// decoder picks the last record.
func TestPollGPU_FromSidecarFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gpu.json")
	body := `{
  "frequency": {"actual": 300},
  "engines": {"Render/3D": {"busy": 12.5}, "Video": {"busy": 0}},
  "power": {"GPU": 1.1},
  "rc6": {"value": 85}
}
{
  "frequency": {"actual": 450},
  "engines": {"Render/3D": {"busy": 33.0}, "Video": {"busy": 5.5}},
  "power": {"GPU": 2.2},
  "rc6": {"value": 60}
}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOMELAB_INTEL_GPU_STATS", path)
	got := pollGPU("")
	if !got.Available {
		t.Fatalf("expected Available=true, got %+v", got)
	}
	if got.FreqMHz != 450 {
		t.Errorf("FreqMHz: got %v want 450 (last record)", got.FreqMHz)
	}
	if got.RenderBusy != 33.0 {
		t.Errorf("RenderBusy: got %v want 33", got.RenderBusy)
	}
}

func TestPollGPU_FileMissing_UsesCache(t *testing.T) {
	lastGoodGPU = GpuStats{FreqMHz: 999, Available: true}
	t.Setenv("HOMELAB_INTEL_GPU_STATS", "/nonexistent/path/gpu.json")
	got := pollGPU("")
	if !got.Available || got.FreqMHz != 999 {
		t.Errorf("expected cached value, got %+v", got)
	}
}
