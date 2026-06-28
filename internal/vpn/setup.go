package vpn

// Params is the subset of VPNConfig needed to assemble setup commands.
type Params struct {
	NetnsName, Interface, Address, DNS      string
	PeerPublicKey, PeerEndpoint, AllowedIPs string
	PrivateKeyFile, VethHostIP, VethNetnsIP string
}

// BuildSetupCommands returns the ordered argv slices to (re)establish the
// netns, the WireGuard interface, the veth pair, and routing. Each slice
// is fed to cmdrunner by the caller. The function is pure and idempotent
// in intent (callers ignore "exists" failures on the create steps).
func BuildSetupCommands(p Params) [][]string {
	ns := p.NetnsName
	wg := p.Interface
	return [][]string{
		// 1. named netns
		{"ip", "netns", "add", ns},
		// 2. wg interface in root ns, then move into the netns
		{"ip", "link", "add", wg, "type", "wireguard"},
		{"ip", "link", "set", wg, "netns", ns},
		// 3. configure wg inside the netns (key read from file, never argv)
		{"ip", "netns", "exec", ns, "wg", "set", wg, "private-key", p.PrivateKeyFile,
			"peer", p.PeerPublicKey, "endpoint", p.PeerEndpoint, "allowed-ips", p.AllowedIPs},
		// 4. address + bring up + default route inside the netns
		{"ip", "netns", "exec", ns, "ip", "address", "add", p.Address, "dev", wg},
		{"ip", "netns", "exec", ns, "ip", "link", "set", wg, "up"},
		{"ip", "netns", "exec", ns, "ip", "route", "add", "default", "dev", wg},
		{"ip", "netns", "exec", ns, "ip", "link", "set", "lo", "up"},
		// 5. veth pair: host side stays in root ns, peer goes into the netns
		{"ip", "link", "add", "veth-host", "type", "veth", "peer", "name", "veth-vpn"},
		{"ip", "link", "set", "veth-vpn", "netns", ns},
		{"ip", "address", "add", p.VethHostIP, "dev", "veth-host"},
		{"ip", "link", "set", "veth-host", "up"},
		{"ip", "netns", "exec", ns, "ip", "address", "add", p.VethNetnsIP, "dev", "veth-vpn"},
		{"ip", "netns", "exec", ns, "ip", "link", "set", "veth-vpn", "up"},
	}
}
