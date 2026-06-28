//go:build integration

package vpn

import (
	"context"
	"testing"
	"time"
)

// TestSetupIntegration requires root + wireguard-tools + iproute2 and a
// valid ProtonVPN key. Run with: go test -tags integration ./internal/vpn/
func TestSetupIntegration(t *testing.T) {
	m := New(Config{ /* fill from a real test config */ Enabled: true})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	m.setup(ctx)
	if ts, err := m.checkHandshake(ctx); err != nil || ts == 0 {
		t.Fatalf("no handshake after setup: ts=%d err=%v", ts, err)
	}
}
