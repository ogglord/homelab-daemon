// Package vpn owns the daemon's WireGuard network namespace: bring-up,
// NAT-PMP port forwarding, health watchdog, and status publishing.
package vpn

import (
	"fmt"
	"regexp"
	"strconv"
)

var mappedPortRe = regexp.MustCompile(`Mapped public port (\d+) protocol`)

// ParseMappedPort extracts the mapped public port from natpmpc stdout.
func ParseMappedPort(output string) (int, error) {
	m := mappedPortRe.FindStringSubmatch(output)
	if m == nil {
		return 0, fmt.Errorf("no mapped public port in natpmpc output")
	}
	return strconv.Atoi(m[1])
}
