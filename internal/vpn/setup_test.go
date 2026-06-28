package vpn

import (
	"strings"
	"testing"
)

func TestBuildSetupCommands(t *testing.T) {
	p := Params{
		NetnsName:      "vpn",
		Interface:      "wg0",
		Address:        "10.2.0.2/32",
		DNS:            "10.2.0.1",
		PeerPublicKey:  "PUB==",
		PeerEndpoint:   "1.2.3.4:51820",
		AllowedIPs:     "0.0.0.0/0",
		PrivateKeyFile: "/run/secrets/vpn/WG_PRIVATE_KEY",
		VethHostIP:     "10.200.0.1/30",
		VethNetnsIP:    "10.200.0.2/30",
	}
	cmds := BuildSetupCommands(p)
	joined := make([]string, len(cmds))
	for i, c := range cmds {
		joined[i] = strings.Join(c, " ")
	}
	all := strings.Join(joined, "\n")

	if joined[0] != "ip netns add vpn" {
		t.Fatalf("first cmd = %q, want netns add", joined[0])
	}
	if !strings.Contains(all, "wg set wg0 private-key /run/secrets/vpn/WG_PRIVATE_KEY") {
		t.Fatalf("missing wg set private-key from file:\n%s", all)
	}
	if !strings.Contains(all, "peer PUB== endpoint 1.2.3.4:51820 allowed-ips 0.0.0.0/0") {
		t.Fatalf("missing peer config:\n%s", all)
	}
	if !strings.Contains(all, "ip link add veth-host type veth peer name veth-vpn") {
		t.Fatalf("missing veth pair:\n%s", all)
	}
	if !strings.Contains(all, "ip netns exec vpn ip route add default dev wg0") {
		t.Fatalf("missing default route:\n%s", all)
	}
}
