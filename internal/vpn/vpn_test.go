package vpn

import "testing"

func TestParamsFromConfig(t *testing.T) {
	m := New(Config{
		Enabled:        true,
		NetnsName:      "vpn",
		Interface:      "wg0",
		Address:        "10.2.0.2/32",
		PrivateKeyFile: "/run/secrets/vpn/WG_PRIVATE_KEY",
		PeerPublicKey:  "PUB==",
		PeerEndpoint:   "1.2.3.4:51820",
		AllowedIPs:     "0.0.0.0/0",
		VethHostIP:     "10.200.0.1/30",
		VethNetnsIP:    "10.200.0.2/30",
	})
	p := m.paramsFromConfig()
	if p.Interface != "wg0" || p.NetnsName != "vpn" || p.VethNetnsIP != "10.200.0.2/30" {
		t.Fatalf("bad params: %+v", p)
	}
}

func TestGetReflectsConfigWhenDisabled(t *testing.T) {
	m := New(Config{Enabled: false, Provider: "protonvpn", Type: "wireguard"})
	s := m.Get()
	if s.Enabled {
		t.Fatal("expected Enabled=false")
	}
	if s.Provider != "protonvpn" || s.Type != "wireguard" {
		t.Fatalf("provider/type not seeded: %+v", s)
	}
}
