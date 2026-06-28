package api

// VPNStatus is the daemon's VPN snapshot, surfaced on /api/vpn and the
// dashboard Overview. JSON tags must match internal/vpn.Status.
type VPNStatus struct {
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
