//go:build !linux

package vms

import "errors"

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

type VMInfo struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Memory string `json:"memory"`
	CPUs   uint   `json:"cpus"`
}

type Module struct{}

func New(_ string) *Module {
	return &Module{}
}

func (m *Module) Close() {}

func (m *Module) GetVMs() ([]VMInfo, error) {
	return nil, nil
}

func (m *Module) RunAction(_, _ string) error {
	return errors.New("VM actions are not supported on this platform")
}
