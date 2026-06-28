package main

import "testing"

func TestParseConfigVPN(t *testing.T) {
	yaml := `
version: 1
vpn:
  enabled: true
  netns_name: vpn
  interface: wg0
  address: 10.2.0.2/32
  dns: 10.2.0.1
  peer_public_key: PUBKEY==
  peer_endpoint: 1.2.3.4:51820
  allowed_ips: 0.0.0.0/0
  private_key_file: /run/secrets/vpn/WG_PRIVATE_KEY
  provider: protonvpn
  type: wireguard
  server_country: Switzerland
  veth_host_ip: 10.200.0.1/30
  veth_netns_ip: 10.200.0.2/30
  port_file: /run/homelab-daemon/vpn/forwarded-port
  refresh_interval_seconds: 45
`
	cfg, err := parseConfig([]byte(yaml))
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if !cfg.VPN.Enabled || cfg.VPN.Interface != "wg0" || cfg.VPN.RefreshIntervalSeconds != 45 {
		t.Fatalf("unexpected VPN config: %+v", cfg.VPN)
	}
	if cfg.VPN.PortFile != "/run/homelab-daemon/vpn/forwarded-port" {
		t.Fatalf("bad port_file: %q", cfg.VPN.PortFile)
	}
}
