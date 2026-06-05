//go:build linux

package vms

import (
	"fmt"
	"sync"

	logging "github.com/ogglord/homelab-logging"
	"libvirt.org/go/libvirt"
)

var log = logging.Logger("api")

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
	Memory string `json:"memory"` // e.g. "4.0 GB"
	CPUs   uint   `json:"cpus"`
}

type Module struct {
	uri  string
	conn *libvirt.Connect
	mu   sync.Mutex
}

func New(uri string) *Module {
	if uri == "" {
		uri = "qemu:///system"
	}
	return &Module{
		uri: uri,
	}
}

func (m *Module) connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.conn != nil {
		return nil
	}
	c, err := libvirt.NewConnect(m.uri)
	if err != nil {
		return fmt.Errorf("libvirt connect: %w", err)
	}
	m.conn = c
	return nil
}

func (m *Module) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.conn != nil {
		_, _ = m.conn.Close()
		m.conn = nil
	}
}

func (m *Module) GetVMs() ([]VMInfo, error) {
	if err := m.connect(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	conn := m.conn
	m.mu.Unlock()

	domains, err := conn.ListAllDomains(
		libvirt.CONNECT_LIST_DOMAINS_ACTIVE | libvirt.CONNECT_LIST_DOMAINS_INACTIVE,
	)
	if err != nil {
		return nil, fmt.Errorf("list domains: %w", err)
	}

	var res []VMInfo
	for i := range domains {
		dom := &domains[i]
		name, _ := dom.GetName()
		state, _, _ := dom.GetState()
		info, _ := dom.GetInfo()

		res = append(res, VMInfo{
			Name:   name,
			State:  stateToString(state),
			Memory: formatMemory(info.Memory),
			CPUs:   info.NrVirtCpu,
		})

		_ = dom.Free()
	}
	return res, nil
}

func (m *Module) RunAction(name, action string) error {
	if err := m.connect(); err != nil {
		return err
	}
	m.mu.Lock()
	conn := m.conn
	m.mu.Unlock()

	dom, err := conn.LookupDomainByName(name)
	if err != nil {
		return fmt.Errorf("lookup domain %s: %w", name, err)
	}
	defer func() {
		_ = dom.Free()
	}()

	log.Info("Running VM action", "name", name, "action", action)

	switch action {
	case "start":
		return dom.Create()
	case "shutdown":
		return dom.Shutdown()
	case "destroy":
		return dom.Destroy()
	case "suspend":
		return dom.Suspend()
	case "resume":
		return dom.Resume()
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

func stateToString(state libvirt.DomainState) string {
	switch state {
	case libvirt.DOMAIN_RUNNING:
		return VMStateRunning
	case libvirt.DOMAIN_SHUTOFF:
		return VMStateShutOff
	case libvirt.DOMAIN_PAUSED:
		return VMStatePaused
	case libvirt.DOMAIN_SHUTDOWN:
		return VMStateShuttingDown
	case libvirt.DOMAIN_CRASHED:
		return VMStateCrashed
	case libvirt.DOMAIN_NOSTATE:
		return VMStateNoState
	case libvirt.DOMAIN_BLOCKED:
		return VMStateBlocked
	case libvirt.DOMAIN_PMSUSPENDED:
		return VMStatePMSuspended
	default:
		return VMStateUnknown
	}
}

func formatMemory(memKiB uint64) string {
	memMiB := float64(memKiB) / 1024.0
	if memMiB >= 1024.0 {
		return fmt.Sprintf("%.1f GB", memMiB/1024.0)
	}
	return fmt.Sprintf("%.0f MB", memMiB)
}
