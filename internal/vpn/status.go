package vpn

import "sync"

// Status is the point-in-time VPN snapshot. JSON tags match api.VPNStatus.
type Status struct {
	Enabled       bool   `json:"Enabled"`
	Connected     bool   `json:"Connected"`
	Provider      string `json:"Provider"`
	Type          string `json:"Type"`
	ServerCountry string `json:"ServerCountry"`
	PublicIP      string `json:"PublicIP"`
	Country       string `json:"Country"`
	ForwardedPort int    `json:"ForwardedPort"`
	LastHandshake string `json:"LastHandshake"`
	FailureCount  int    `json:"FailureCount"`
	ErrMsg        string `json:"ErrMsg,omitempty"`
}

type stateCache struct {
	mu sync.RWMutex
	s  Status
}

func newStateCache() *stateCache { return &stateCache{} }

func (c *stateCache) set(s Status) {
	c.mu.Lock()
	c.s = s
	c.mu.Unlock()
}

func (c *stateCache) get() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.s
}
