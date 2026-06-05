package api

// VMInfo is the body of GET /api/v1/vms (one element per VM).
type VMInfo struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Memory string `json:"memory"` // human-readable, e.g. "4.0 GB"
	CPUs   uint   `json:"cpus"`
}

// Canonical VM-state strings, mirrored on the frontend.
const (
	VMStateRunning      = "Running"
	VMStatePaused       = "Paused"
	VMStateShutOff      = "Shut Off"
	VMStateShuttingDown = "Shutting Down"
	VMStateCrashed      = "Crashed"
	VMStateNoState      = "No State"
	VMStateBlocked      = "Blocked"
	VMStatePMSuspended  = "PM Suspended"
	VMStateUnknown      = "Unknown"
)
